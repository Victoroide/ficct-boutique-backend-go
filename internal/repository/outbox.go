package repository

import (
	"context"
	"time"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepo(pool *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{pool: pool}
}

func (r *OutboxRepo) Enqueue(ctx context.Context, tx pgx.Tx, eventType string, aggregateID uuid.UUID, payload []byte, targetURL string) (*models.WebhookEvent, error) {
	const q = `INSERT INTO webhook_outbox (event_type, aggregate_id, payload, target_url)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING id, event_type, aggregate_id, payload, target_url, status, attempts, last_error, next_attempt_at, delivered_at, created_at`
	e := &models.WebhookEvent{}
	err := tx.QueryRow(ctx, q, eventType, aggregateID, string(payload), targetURL).Scan(
		&e.ID, &e.EventType, &e.AggregateID, &e.Payload, &e.TargetURL, &e.Status, &e.Attempts, &e.LastError, &e.NextAttemptAt, &e.DeliveredAt, &e.CreatedAt,
	)
	return e, err
}

func (r *OutboxRepo) FetchDue(ctx context.Context, limit int) ([]models.WebhookEvent, error) {
	const q = `SELECT id, event_type, aggregate_id, payload, target_url, status, attempts, last_error, next_attempt_at, delivered_at, created_at
		FROM webhook_outbox
		WHERE status = 'pending' AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := []models.WebhookEvent{}
	for rows.Next() {
		e := models.WebhookEvent{}
		if err := rows.Scan(&e.ID, &e.EventType, &e.AggregateID, &e.Payload, &e.TargetURL, &e.Status, &e.Attempts, &e.LastError, &e.NextAttemptAt, &e.DeliveredAt, &e.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OutboxRepo) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE webhook_outbox SET status = 'delivered', delivered_at = NOW(), last_error = NULL WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id)
	return err
}

func (r *OutboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, errMsg string, nextAttemptAt time.Time, terminal bool) error {
	status := "pending"
	if terminal {
		status = "failed"
	}
	const q = `UPDATE webhook_outbox SET status = $2, attempts = $3, last_error = $4, next_attempt_at = $5 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, status, attempts, errMsg, nextAttemptAt)
	return err
}
