package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ficct-boutique/backend-go/internal/auth"
	"github.com/ficct-boutique/backend-go/internal/middleware"
	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/service"
)

// fakeStore is a lightweight in-memory PushTokenStore used by the resolver
// RBAC tests. We avoid touching Postgres so these tests run in plain `go
// test ./graph/...` with no fixtures.
type fakeStore struct {
	tokens      []models.PushToken
	deactivated []string
}

func (f *fakeStore) ListAllActive(_ context.Context) ([]models.PushToken, error) {
	out := []models.PushToken{}
	for _, t := range f.tokens {
		if t.IsActive {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeStore) ListActiveByUser(_ context.Context, userID uuid.UUID) ([]models.PushToken, error) {
	out := []models.PushToken{}
	for _, t := range f.tokens {
		if t.IsActive && t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeStore) ListActiveForUsers(_ context.Context, ids []uuid.UUID) ([]models.PushToken, error) {
	set := map[uuid.UUID]struct{}{}
	for _, id := range ids {
		set[id] = struct{}{}
	}
	out := []models.PushToken{}
	for _, t := range f.tokens {
		if _, ok := set[t.UserID]; ok && t.IsActive {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeStore) DeactivateByToken(_ context.Context, token string) error {
	for i := range f.tokens {
		if f.tokens[i].Token == token {
			f.tokens[i].IsActive = false
		}
	}
	f.deactivated = append(f.deactivated, token)
	return nil
}

// startFakeExpo returns an httptest server that always returns "ok" tickets,
// and a cleanup func. It also returns a pointer to the captured payloads so
// tests can assert on what the resolver sent.
func startFakeExpo(t *testing.T) (*httptest.Server, *[][]service.ExpoMessage) {
	t.Helper()
	captured := &[][]service.ExpoMessage{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []service.ExpoMessage
		_ = json.NewDecoder(r.Body).Decode(&batch)
		*captured = append(*captured, batch)
		tickets := make([]service.ExpoTicket, len(batch))
		for i := range tickets {
			tickets[i] = service.ExpoTicket{Status: "ok", ID: uuid.NewString()}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Data []service.ExpoTicket `json:"data"`
		}{Data: tickets})
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

// claimsFor builds a Claims object for the given role + subject. The
// resolvers only care about Role and Subject; everything else is filler.
func claimsFor(role auth.Role, sub uuid.UUID) *auth.Claims {
	return &auth.Claims{
		Role:  role,
		Email: string(role) + "@example.test",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: sub.String(),
		},
	}
}

// resolverWithStore builds a Resolver that has only the push sender wired —
// other repos remain nil because these tests do not exercise them.
func resolverWithStore(t *testing.T, store *fakeStore) (*Resolver, *[][]service.ExpoMessage) {
	srv, captured := startFakeExpo(t)
	sender := service.NewPushSender(srv.URL, "", store)
	return &Resolver{PushSender: sender}, captured
}

// ---- RBAC ----

func TestSendPushCampaign_RejectsAnonymous(t *testing.T) {
	r, _ := resolverWithStore(t, &fakeStore{})
	_, err := r.SendPushCampaign(context.Background(), struct {
		Input struct {
			Title   string
			Body    string
			UserIDs *[]UUID
		}
	}{Input: struct {
		Title   string
		Body    string
		UserIDs *[]UUID
	}{Title: "hi", Body: "x"}})
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestSendPushCampaign_RejectsCustomer(t *testing.T) {
	r, _ := resolverWithStore(t, &fakeStore{})
	ctx := middleware.ContextWithClaims(context.Background(), claimsFor(auth.RoleCustomer, uuid.New()))
	_, err := r.SendPushCampaign(ctx, struct {
		Input struct {
			Title   string
			Body    string
			UserIDs *[]UUID
		}
	}{Input: struct {
		Title   string
		Body    string
		UserIDs *[]UUID
	}{Title: "hi", Body: "x"}})
	require.ErrorIs(t, err, ErrForbidden, "customers must not be able to trigger campaigns")
}

func TestSendPushCampaign_RejectsStaff(t *testing.T) {
	r, _ := resolverWithStore(t, &fakeStore{})
	ctx := middleware.ContextWithClaims(context.Background(), claimsFor(auth.RoleStaff, uuid.New()))
	_, err := r.SendPushCampaign(ctx, struct {
		Input struct {
			Title   string
			Body    string
			UserIDs *[]UUID
		}
	}{Input: struct {
		Title   string
		Body    string
		UserIDs *[]UUID
	}{Title: "hi", Body: "x"}})
	require.ErrorIs(t, err, ErrForbidden, "staff must not be able to trigger campaigns either")
}

func TestSendPushCampaign_AdminCanSendAndPayloadReachesExpo(t *testing.T) {
	target := uuid.New()
	store := &fakeStore{
		tokens: []models.PushToken{
			{Token: "ExponentPushToken[abc]", IsActive: true, UserID: target},
		},
	}
	r, captured := resolverWithStore(t, store)
	ctx := middleware.ContextWithClaims(context.Background(), claimsFor(auth.RoleAdmin, uuid.New()))

	ids := []UUID{UUIDFrom(target)}
	res, err := r.SendPushCampaign(ctx, struct {
		Input struct {
			Title   string
			Body    string
			UserIDs *[]UUID
		}
	}{Input: struct {
		Title   string
		Body    string
		UserIDs *[]UUID
	}{Title: "Promo Otoño", Body: "Hasta 30% off", UserIDs: &ids}})

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int32(1), res.Sent_)
	require.Equal(t, int32(0), res.Failed_)

	// Fake Expo received exactly the one targeted message.
	require.Len(t, *captured, 1, "exactly one HTTP POST to fake Expo")
	require.Len(t, (*captured)[0], 1, "exactly one message in the batch")
	msg := (*captured)[0][0]
	require.Equal(t, "ExponentPushToken[abc]", msg.To)
	require.Equal(t, "Promo Otoño", msg.Title)
	require.Equal(t, "Hasta 30% off", msg.Body)
}

func TestSendTestPushNotification_AnyAuthCanFire_ButOnlyOwnTokens(t *testing.T) {
	caller := uuid.New()
	other := uuid.New()
	store := &fakeStore{
		tokens: []models.PushToken{
			{Token: "ExponentPushToken[caller]", IsActive: true, UserID: caller},
			{Token: "ExponentPushToken[other]", IsActive: true, UserID: other},
		},
	}
	r, captured := resolverWithStore(t, store)
	ctx := middleware.ContextWithClaims(context.Background(), claimsFor(auth.RoleCustomer, caller))

	res, err := r.SendTestPushNotification(ctx, struct{ Title, Body string }{Title: "ping", Body: "pong"})
	require.NoError(t, err)
	require.Equal(t, int32(1), res.Sent_, "test mutation must only reach the caller's own tokens, never anybody else's")

	require.Len(t, *captured, 1)
	require.Len(t, (*captured)[0], 1)
	require.Equal(t, "ExponentPushToken[caller]", (*captured)[0][0].To)
}

func TestSendTestPushNotification_RejectsAnonymous(t *testing.T) {
	r, _ := resolverWithStore(t, &fakeStore{})
	_, err := r.SendTestPushNotification(context.Background(), struct{ Title, Body string }{Title: "x", Body: "y"})
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestSendTestPushNotification_RejectsEmptyTitle(t *testing.T) {
	r, _ := resolverWithStore(t, &fakeStore{})
	ctx := middleware.ContextWithClaims(context.Background(), claimsFor(auth.RoleCustomer, uuid.New()))
	_, err := r.SendTestPushNotification(ctx, struct{ Title, Body string }{Title: "  ", Body: "y"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}
