package repository

import (
	"context"
	"errors"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BranchRepo struct {
	pool *pgxpool.Pool
}

func NewBranchRepo(pool *pgxpool.Pool) *BranchRepo {
	return &BranchRepo{pool: pool}
}

func (r *BranchRepo) Create(ctx context.Context, b *models.Branch) (*models.Branch, error) {
	const q = `INSERT INTO branches (code, name, address, latitude, longitude, phone)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, code, name, address, latitude, longitude, phone, is_active, created_at, updated_at`
	out := &models.Branch{}
	err := r.pool.QueryRow(ctx, q, b.Code, b.Name, b.Address, b.Latitude, b.Longitude, b.Phone).Scan(
		&out.ID, &out.Code, &out.Name, &out.Address, &out.Latitude, &out.Longitude, &out.Phone, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	return out, err
}

func (r *BranchRepo) List(ctx context.Context) ([]models.Branch, error) {
	const q = `SELECT id, code, name, address, latitude, longitude, phone, is_active, created_at, updated_at
		FROM branches WHERE is_active = TRUE ORDER BY name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Branch{}
	for rows.Next() {
		b := models.Branch{}
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Address, &b.Latitude, &b.Longitude, &b.Phone, &b.IsActive, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *BranchRepo) Find(ctx context.Context, id uuid.UUID) (*models.Branch, error) {
	const q = `SELECT id, code, name, address, latitude, longitude, phone, is_active, created_at, updated_at
		FROM branches WHERE id = $1`
	b := &models.Branch{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&b.ID, &b.Code, &b.Name, &b.Address, &b.Latitude, &b.Longitude, &b.Phone, &b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}
