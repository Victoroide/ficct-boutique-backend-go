package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// fakeStore is an in-memory implementation of PushTokenStore for tests.
type fakeStore struct {
	mu          sync.Mutex
	tokens      []models.PushToken
	deactivated []string
}

func (f *fakeStore) ListAllActive(_ context.Context) ([]models.PushToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]models.PushToken, 0, len(f.tokens))
	for _, t := range f.tokens {
		if t.IsActive {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeStore) ListActiveByUser(_ context.Context, userID uuid.UUID) ([]models.PushToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.PushToken{}
	for _, t := range f.tokens {
		if t.IsActive && t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeStore) ListActiveForUsers(_ context.Context, ids []uuid.UUID) ([]models.PushToken, error) {
	set := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.PushToken{}
	for _, t := range f.tokens {
		if _, ok := set[t.UserID]; ok && t.IsActive {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeStore) DeactivateByToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.tokens {
		if f.tokens[i].Token == token {
			f.tokens[i].IsActive = false
		}
	}
	f.deactivated = append(f.deactivated, token)
	return nil
}

// fakeExpoServer mimics the subset of Expo's push API the sender uses.
// Good tokens get an `ok` ticket; tokens containing "BAD" come back with
// DeviceNotRegistered. The handler records every payload so tests can assert.
type fakeExpoServer struct {
	mu       sync.Mutex
	received [][]ExpoMessage
	mode     string // "ok", "device_not_registered", "http500", "malformed"
}

func newFakeExpo(mode string) *httptest.Server {
	srv := &fakeExpoServer{mode: mode}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var batch []ExpoMessage
		_ = json.Unmarshal(body, &batch)
		srv.mu.Lock()
		srv.received = append(srv.received, batch)
		srv.mu.Unlock()

		switch srv.mode {
		case "http500":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"code":"PUSH_INTERNAL"}]}`))
			return
		case "malformed":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<not-json>`))
			return
		}

		tickets := make([]ExpoTicket, 0, len(batch))
		for _, m := range batch {
			if srv.mode == "device_not_registered" {
				tickets = append(tickets, ExpoTicket{
					Status:  "error",
					Message: "\"" + m.To + "\" is not a registered push notification recipient",
					Details: &struct {
						Error string `json:"error"`
					}{Error: "DeviceNotRegistered"},
				})
				continue
			}
			tickets = append(tickets, ExpoTicket{Status: "ok", ID: uuid.NewString()})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expoTicketResponse{Data: tickets})
	}))
}

// ---- tests ----

func TestPushSender_SendsToTokens_HappyPath(t *testing.T) {
	srv := newFakeExpo("ok")
	defer srv.Close()
	store := &fakeStore{}
	sender := NewPushSender(srv.URL, "", store)

	res, err := sender.SendToTokens(context.Background(),
		[]string{"ExponentPushToken[abc]", "ExponentPushToken[def]"},
		"Hola", "Cuerpo", map[string]interface{}{"kind": "test"})
	require.NoError(t, err)
	require.Equal(t, 2, res.Sent)
	require.Equal(t, 0, res.Failed)
	require.Equal(t, 0, res.Deactivated)
	require.Len(t, res.Tickets, 2)
	require.Equal(t, "ok", res.Tickets[0].Status)
}

func TestPushSender_DeactivatesTokenOnDeviceNotRegistered(t *testing.T) {
	srv := newFakeExpo("device_not_registered")
	defer srv.Close()
	store := &fakeStore{
		tokens: []models.PushToken{
			{Token: "ExponentPushToken[dead]", IsActive: true},
		},
	}
	sender := NewPushSender(srv.URL, "", store)

	res, err := sender.SendToTokens(context.Background(), []string{"ExponentPushToken[dead]"}, "x", "y", nil)
	require.NoError(t, err)
	require.Equal(t, 0, res.Sent)
	require.Equal(t, 1, res.Failed)
	require.Equal(t, 1, res.Deactivated)
	require.Contains(t, store.deactivated, "ExponentPushToken[dead]")
	require.False(t, store.tokens[0].IsActive, "store should now mark the token inactive")
}

func TestPushSender_FailsClosedOnHTTP500(t *testing.T) {
	srv := newFakeExpo("http500")
	defer srv.Close()
	store := &fakeStore{}
	sender := NewPushSender(srv.URL, "", store)

	res, err := sender.SendToTokens(context.Background(), []string{"ExponentPushToken[abc]"}, "x", "y", nil)
	require.NoError(t, err, "non-fatal: errors come back via the result, not as Go error")
	require.Equal(t, 0, res.Sent)
	require.Equal(t, 1, res.Failed)
	require.NotEmpty(t, res.Errors)
}

func TestPushSender_HandlesMalformedResponse(t *testing.T) {
	srv := newFakeExpo("malformed")
	defer srv.Close()
	store := &fakeStore{}
	sender := NewPushSender(srv.URL, "", store)

	res, err := sender.SendToTokens(context.Background(), []string{"ExponentPushToken[abc]"}, "x", "y", nil)
	require.NoError(t, err)
	require.Equal(t, 1, res.Failed)
	require.NotEmpty(t, res.Errors)
}

func TestPushSender_BatchesAt100(t *testing.T) {
	var captured [][]ExpoMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var batch []ExpoMessage
		_ = json.Unmarshal(body, &batch)
		captured = append(captured, batch)
		tickets := make([]ExpoTicket, len(batch))
		for i := range tickets {
			tickets[i] = ExpoTicket{Status: "ok", ID: uuid.NewString()}
		}
		_ = json.NewEncoder(w).Encode(expoTicketResponse{Data: tickets})
	}))
	defer srv.Close()
	sender := NewPushSender(srv.URL, "", &fakeStore{})

	tokens := make([]string, 0, 250)
	for i := 0; i < 250; i++ {
		tokens = append(tokens, "ExponentPushToken[abc]")
	}
	res, err := sender.SendToTokens(context.Background(), tokens, "x", "y", nil)
	require.NoError(t, err)
	require.Equal(t, 250, res.Sent)
	require.Len(t, captured, 3, "expected 3 batches: 100 + 100 + 50")
	require.Len(t, captured[0], 100)
	require.Len(t, captured[1], 100)
	require.Len(t, captured[2], 50)
}

func TestPushSender_SendCampaignToUsers_FiltersByUserID(t *testing.T) {
	srv := newFakeExpo("ok")
	defer srv.Close()
	wantedUser := uuid.New()
	otherUser := uuid.New()
	store := &fakeStore{
		tokens: []models.PushToken{
			{Token: "ExponentPushToken[wanted-1]", IsActive: true, UserID: wantedUser},
			{Token: "ExponentPushToken[wanted-2]", IsActive: true, UserID: wantedUser},
			{Token: "ExponentPushToken[other]", IsActive: true, UserID: otherUser},
			{Token: "ExponentPushToken[inactive]", IsActive: false, UserID: wantedUser},
		},
	}
	sender := NewPushSender(srv.URL, "", store)

	res, err := sender.SendCampaignToUsers(context.Background(), []uuid.UUID{wantedUser}, "promo", "Otoño 2026", nil)
	require.NoError(t, err)
	require.Equal(t, 2, res.Sent, "only the two active tokens for the wanted user should be sent")
}

func TestPushSender_SendTestToCaller_OnlyCallerTokens(t *testing.T) {
	srv := newFakeExpo("ok")
	defer srv.Close()
	caller := uuid.New()
	other := uuid.New()
	store := &fakeStore{
		tokens: []models.PushToken{
			{Token: "ExponentPushToken[caller]", IsActive: true, UserID: caller},
			{Token: "ExponentPushToken[other]", IsActive: true, UserID: other},
		},
	}
	sender := NewPushSender(srv.URL, "", store)
	res, err := sender.SendTestToCaller(context.Background(), caller, "hola", "mundo")
	require.NoError(t, err)
	require.Equal(t, 1, res.Sent)
}

func TestPushSender_NoTokensIsNotAnError(t *testing.T) {
	srv := newFakeExpo("ok")
	defer srv.Close()
	sender := NewPushSender(srv.URL, "", &fakeStore{})

	res, err := sender.SendToTokens(context.Background(), nil, "t", "b", nil)
	require.NoError(t, err)
	require.Equal(t, 0, res.Sent)
	require.Equal(t, 0, res.Failed)
}

func TestPushSender_AttachesAccessTokenHeader(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(expoTicketResponse{Data: []ExpoTicket{{Status: "ok", ID: "x"}}})
	}))
	defer srv.Close()
	sender := NewPushSender(srv.URL, "super-secret", &fakeStore{})
	_, err := sender.SendToTokens(context.Background(), []string{"t"}, "a", "b", nil)
	require.NoError(t, err)
	require.Equal(t, "Bearer super-secret", seenAuth)
}
