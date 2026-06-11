package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ficct-boutique/backend-go/internal/models"
)

var ErrNotFound = errors.New("not found")

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `SELECT id, email, password_hash, full_name, role, is_active, created_at, updated_at
		FROM users WHERE LOWER(email) = LOWER($1) LIMIT 1`
	u := &models.User{}
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `SELECT id, email, password_hash, full_name, role, is_active, created_at, updated_at
		FROM users WHERE id = $1`
	u := &models.User{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) Create(ctx context.Context, email, hash, fullName string, role models.Role) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	const q = `INSERT INTO users (email, password_hash, full_name, role)
		VALUES ($1,$2,$3,$4)
		RETURNING id, email, password_hash, full_name, role, is_active, created_at, updated_at`
	u := &models.User{}
	err := r.pool.QueryRow(ctx, q, email, hash, fullName, role).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

func (r *UserRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET updated_at = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, time.Now().UTC())
	return err
}
