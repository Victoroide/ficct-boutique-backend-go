package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/ficct-boutique/backend-go/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Dispatcher struct {
	outbox     *repository.OutboxRepo
	httpClient *http.Client
	secret     string
	interval   time.Duration
	maxRetries int
}

func NewDispatcher(outbox *repository.OutboxRepo, secret string, interval time.Duration, maxRetries int) *Dispatcher {
	return &Dispatcher{
		outbox: outbox,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		secret:     secret,
		interval:   interval,
		maxRetries: maxRetries,
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	log.Info().Dur("interval", d.interval).Msg("webhook dispatcher started")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("webhook dispatcher stopped")
			return
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

func (d *Dispatcher) tick(ctx context.Context) {
	events, err := d.outbox.FetchDue(ctx, 10)
	if err != nil {
		log.Error().Err(err).Msg("fetch due webhooks")
		return
	}
	for _, e := range events {
		d.deliver(ctx, e.ID, e.EventType, e.TargetURL, e.Payload, e.Attempts)
	}
}

func (d *Dispatcher) deliver(ctx context.Context, id uuid.UUID, eventType, target string, payload []byte, prevAttempts int) {
	attempts := prevAttempts + 1
	signature := sign(d.secret, payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		d.markFailure(ctx, id, attempts, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FICCT-Event", eventType)
	req.Header.Set("X-FICCT-Signature", "sha256="+signature)
	req.Header.Set("X-FICCT-Event-Id", id.String())

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.markFailure(ctx, id, attempts, err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := d.outbox.MarkDelivered(ctx, id); err != nil {
			log.Error().Err(err).Str("id", id.String()).Msg("mark delivered")
		} else {
			log.Info().Str("id", id.String()).Str("event", eventType).Int("status", resp.StatusCode).Msg("webhook delivered")
		}
		return
	}

	msg := fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body))
	d.markFailure(ctx, id, attempts, msg)
}

func (d *Dispatcher) markFailure(ctx context.Context, id uuid.UUID, attempts int, msg string) {
	terminal := attempts >= d.maxRetries
	backoff := time.Duration(math.Pow(2, float64(attempts))) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	next := time.Now().Add(backoff)
	if err := d.outbox.MarkFailed(ctx, id, attempts, msg, next, terminal); err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("mark failed")
	}
	log.Warn().Str("id", id.String()).Int("attempts", attempts).Bool("terminal", terminal).Str("error", msg).Msg("webhook delivery failed")
}

func sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify is exposed for receivers (e.g. tests, integration consumers).
func Verify(secret string, payload []byte, sig string) bool {
	want := sign(secret, payload)
	return hmac.Equal([]byte(want), []byte(sig))
}
