package repository

import (
	"context"
	"errors"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepo struct {
	pool *pgxpool.Pool
}

func NewOrderRepo(pool *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{pool: pool}
}

func (r *OrderRepo) CreateForSale(ctx context.Context, tx pgx.Tx, saleID uuid.UUID, code string) (*models.Order, error) {
	const q = `INSERT INTO orders (sale_id, code) VALUES ($1, $2)
		RETURNING id, sale_id, code, status, notes, created_at, updated_at`
	o := &models.Order{}
	err := tx.QueryRow(ctx, q, saleID, code).Scan(
		&o.ID, &o.SaleID, &o.Code, &o.Status, &o.Notes, &o.CreatedAt, &o.UpdatedAt,
	)
	return o, err
}

func (r *OrderRepo) List(ctx context.Context, status *string, limit, offset int) ([]models.Order, error) {
	q := `SELECT id, sale_id, code, status, notes, created_at, updated_at FROM orders WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if status != nil && *status != "" {
		q += " AND status = $" + itoa(idx)
		args = append(args, *status)
		idx++
	}
	q += " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Order{}
	for rows.Next() {
		o := models.Order{}
		if err := rows.Scan(&o.ID, &o.SaleID, &o.Code, &o.Status, &o.Notes, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *OrderRepo) Find(ctx context.Context, id uuid.UUID) (*models.Order, error) {
	const q = `SELECT id, sale_id, code, status, notes, created_at, updated_at FROM orders WHERE id = $1`
	o := &models.Order{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&o.ID, &o.SaleID, &o.Code, &o.Status, &o.Notes, &o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}
