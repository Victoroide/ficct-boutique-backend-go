package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ficct-boutique/backend-go/internal/models"
)

// PushTokenRepo provides data access for device push notification tokens.
type PushTokenRepo struct {
	pool *pgxpool.Pool
}

// NewPushTokenRepo constructs a PushTokenRepo backed by the given connection pool.
func NewPushTokenRepo(pool *pgxpool.Pool) *PushTokenRepo {
	return &PushTokenRepo{pool: pool}
}

// Upsert inserts a new token row or, when the token already exists, takes
// ownership for the given user and re-activates it. The unique index on
// token guarantees a single row per device token.
func (r *PushTokenRepo) Upsert(ctx context.Context, userID uuid.UUID, token string, platform models.PushPlatform, deviceID *string) (*models.PushToken, error) {
	const q = `
		INSERT INTO push_tokens (user_id, token, platform, device_id, is_active, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, TRUE, NOW(), NOW(), NOW())
		ON CONFLICT (token) DO UPDATE
		SET user_id      = EXCLUDED.user_id,
		    platform     = EXCLUDED.platform,
		    device_id    = EXCLUDED.device_id,
		    is_active    = TRUE,
		    last_seen_at = NOW(),
		    updated_at   = NOW()
		RETURNING id, user_id, token, platform, device_id, is_active, last_seen_at, created_at, updated_at
	`
	pt := &models.PushToken{}
	err := r.pool.QueryRow(ctx, q, userID, token, platform, deviceID).Scan(
		&pt.ID, &pt.UserID, &pt.Token, &pt.Platform, &pt.DeviceID, &pt.IsActive, &pt.LastSeenAt, &pt.CreatedAt, &pt.UpdatedAt,
	)
	return pt, err
}

// Deactivate flips is_active=false for the matching token belonging to the
// given user. Cross-user deactivation is silently a no-op so that a stolen
// token cannot be used to disable somebody else's notifications.
func (r *PushTokenRepo) Deactivate(ctx context.Context, userID uuid.UUID, token string) error {
	const q = `UPDATE push_tokens SET is_active = FALSE, updated_at = NOW()
		WHERE token = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, q, token, userID)
	return err
}

// ListActiveByUser returns all currently active tokens for a user. Used by
// the (future) campaign sender; today it powers the "registered devices"
// listing in the notification center.
func (r *PushTokenRepo) ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]models.PushToken, error) {
	const q = `SELECT id, user_id, token, platform, device_id, is_active, last_seen_at, created_at, updated_at
		FROM push_tokens WHERE user_id = $1 AND is_active = TRUE ORDER BY last_seen_at DESC`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.PushToken{}
	for rows.Next() {
		pt := models.PushToken{}
		if err := rows.Scan(&pt.ID, &pt.UserID, &pt.Token, &pt.Platform, &pt.DeviceID, &pt.IsActive, &pt.LastSeenAt, &pt.CreatedAt, &pt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

// DeactivateByToken flips is_active=false ignoring the owner. Used by the
// push sender when Expo replies with DeviceNotRegistered; the token is dead
// and should not be retried by anyone.
func (r *PushTokenRepo) DeactivateByToken(ctx context.Context, token string) error {
	const q = `UPDATE push_tokens SET is_active = FALSE, updated_at = NOW() WHERE token = $1`
	_, err := r.pool.Exec(ctx, q, token)
	return err
}

func (r *PushTokenRepo) ListAllActive(ctx context.Context) ([]models.PushToken, error) {
	const q = `SELECT id, user_id, token, platform, device_id, is_active, last_seen_at, created_at, updated_at
		FROM push_tokens WHERE is_active = TRUE ORDER BY last_seen_at DESC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.PushToken{}
	for rows.Next() {
		pt := models.PushToken{}
		if err := rows.Scan(&pt.ID, &pt.UserID, &pt.Token, &pt.Platform, &pt.DeviceID, &pt.IsActive, &pt.LastSeenAt, &pt.CreatedAt, &pt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

func (r *PushTokenRepo) ListActiveForUsers(ctx context.Context, userIDs []uuid.UUID) ([]models.PushToken, error) {
	if len(userIDs) == 0 {
		return []models.PushToken{}, nil
	}
	const q = `SELECT id, user_id, token, platform, device_id, is_active, last_seen_at, created_at, updated_at
		FROM push_tokens WHERE user_id = ANY($1) AND is_active = TRUE ORDER BY last_seen_at DESC`
	rows, err := r.pool.Query(ctx, q, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.PushToken{}
	for rows.Next() {
		pt := models.PushToken{}
		if err := rows.Scan(&pt.ID, &pt.UserID, &pt.Token, &pt.Platform, &pt.DeviceID, &pt.IsActive, &pt.LastSeenAt, &pt.CreatedAt, &pt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

func (r *PushTokenRepo) FindByToken(ctx context.Context, token string) (*models.PushToken, error) {
	const q = `SELECT id, user_id, token, platform, device_id, is_active, last_seen_at, created_at, updated_at
		FROM push_tokens WHERE token = $1`
	pt := &models.PushToken{}
	err := r.pool.QueryRow(ctx, q, token).Scan(
		&pt.ID, &pt.UserID, &pt.Token, &pt.Platform, &pt.DeviceID, &pt.IsActive, &pt.LastSeenAt, &pt.CreatedAt, &pt.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return pt, err
}
