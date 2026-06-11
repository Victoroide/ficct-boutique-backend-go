package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/repository"
)

// PushSender posts notification payloads to the Expo Push API (or any
// drop-in compatible endpoint, e.g. the fake one used in Docker).
//
// The Expo Push API contract is documented at:
//
//	https://docs.expo.dev/push-notifications/sending-notifications/
//
// Briefly: POST a JSON array of messages to /--/api/v2/push/send and receive
// back a JSON object whose `data` field is an array of tickets, parallel to
// the input. Each ticket is either:
//
//	{"status":"ok","id":"<receipt-id>"}                                  -> success
//	{"status":"error","message":"...","details":{"error":"<expoCode>"}} -> failure
//
// The most actionable expoCode is `DeviceNotRegistered`, which means the
// token is dead (uninstalled app, opted out). We deactivate those tokens so
// future campaigns don't waste calls on them.
type PushSender struct {
	endpoint    string
	accessToken string
	client      *http.Client
	repo        PushTokenStore
}

// PushTokenStore is the subset of repository.PushTokenRepo that the sender
// actually uses. Defining the interface here lets tests substitute a fake.
type PushTokenStore interface {
	ListAllActive(ctx context.Context) ([]models.PushToken, error)
	ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]models.PushToken, error)
	ListActiveForUsers(ctx context.Context, userIDs []uuid.UUID) ([]models.PushToken, error)
	DeactivateByToken(ctx context.Context, token string) error
}

type ExpoMessage struct {
	To       string                 `json:"to"`
	Title    string                 `json:"title,omitempty"`
	Body     string                 `json:"body,omitempty"`
	Sound    string                 `json:"sound,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Priority string                 `json:"priority,omitempty"`
}

type expoTicketResponse struct {
	Data []ExpoTicket `json:"data"`
}

type ExpoTicket struct {
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Details *struct {
		Error string `json:"error"`
	} `json:"details,omitempty"`
}

// SendResult summarizes one campaign run.
type SendResult struct {
	Sent        int
	Failed      int
	Deactivated int
	Errors      []string
	Tickets     []ExpoTicket
}

func NewPushSender(endpoint, accessToken string, repo PushTokenStore) *PushSender {
	return &PushSender{
		endpoint:    endpoint,
		accessToken: accessToken,
		repo:        repo,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

// Endpoint returns the configured Expo endpoint. Useful for debugging /
// for the health surface to confirm the right URL is wired.
func (s *PushSender) Endpoint() string { return s.endpoint }

// SendToTokens fires a single push payload to an explicit list of tokens
// without touching the repository.
//
// Note: Expo rate-limits batches at 100 messages; this method chunks
// automatically so callers don't have to.
func (s *PushSender) SendToTokens(ctx context.Context, tokens []string, title, body string, data map[string]interface{}) (*SendResult, error) {
	if s.endpoint == "" {
		return nil, errors.New("push sender endpoint not configured")
	}
	out := &SendResult{}
	if len(tokens) == 0 {
		return out, nil
	}
	for _, batch := range chunkStrings(tokens, 100) {
		messages := make([]ExpoMessage, 0, len(batch))
		for _, t := range batch {
			messages = append(messages, ExpoMessage{
				To:       t,
				Title:    title,
				Body:     body,
				Sound:    "default",
				Data:     data,
				Priority: "high",
			})
		}
		tickets, err := s.postMessages(ctx, messages)
		if err != nil {
			out.Errors = append(out.Errors, err.Error())
			out.Failed += len(batch)
			continue
		}
		// One ticket per message, same order.
		for i, ticket := range tickets {
			out.Tickets = append(out.Tickets, ticket)
			if ticket.Status == "ok" {
				out.Sent++
				continue
			}
			out.Failed++
			if ticket.Message != "" {
				out.Errors = append(out.Errors, ticket.Message)
			}
			// Deactivate dead tokens so the next campaign skips them.
			if ticket.Details != nil && ticket.Details.Error == "DeviceNotRegistered" && i < len(batch) {
				if err := s.repo.DeactivateByToken(ctx, batch[i]); err == nil {
					out.Deactivated++
				}
			}
		}
	}
	return out, nil
}

// SendCampaignToUsers loads active tokens for the given user IDs and sends.
// Pass an empty slice to broadcast to every active token in the system.
func (s *PushSender) SendCampaignToUsers(ctx context.Context, userIDs []uuid.UUID, title, body string, data map[string]interface{}) (*SendResult, error) {
	var rows []models.PushToken
	var err error
	if len(userIDs) == 0 {
		rows, err = s.repo.ListAllActive(ctx)
	} else {
		rows, err = s.repo.ListActiveForUsers(ctx, userIDs)
	}
	if err != nil {
		return nil, err
	}
	tokens := make([]string, 0, len(rows))
	for _, r := range rows {
		tokens = append(tokens, r.Token)
	}
	return s.SendToTokens(ctx, tokens, title, body, data)
}

// SendTestToCaller sends to every active token belonging to the calling user.
// Safe to wire as a public mutation because it cannot reach anyone else.
func (s *PushSender) SendTestToCaller(ctx context.Context, userID uuid.UUID, title, body string) (*SendResult, error) {
	rows, err := s.repo.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	tokens := make([]string, 0, len(rows))
	for _, r := range rows {
		tokens = append(tokens, r.Token)
	}
	return s.SendToTokens(ctx, tokens, title, body, nil)
}

func (s *PushSender) postMessages(ctx context.Context, messages []ExpoMessage) ([]ExpoTicket, error) {
	raw, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	if s.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.accessToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("push http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("push http status %d: %s", resp.StatusCode, truncate(string(body), 240))
	}
	var parsed expoTicketResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("push parse: %w (body=%s)", err, truncate(string(body), 240))
	}
	return parsed.Data, nil
}

func chunkStrings(in []string, size int) [][]string {
	if size <= 0 {
		size = 100
	}
	out := make([][]string, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		out = append(out, in[i:end])
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// compile-time check: the real repo satisfies the small interface above.
var _ PushTokenStore = (*repository.PushTokenRepo)(nil)
