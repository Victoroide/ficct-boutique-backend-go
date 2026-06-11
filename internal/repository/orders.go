package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ficct-boutique/backend-go/internal/models"
)

// OrderRepo provides data access for fulfillment orders derived from sales.
type OrderRepo struct {
	pool *pgxpool.Pool
}

// NewOrderRepo constructs an OrderRepo backed by the given connection pool.
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

// ListByUser returns the orders whose underlying sale belongs to the customer
// profile linked to the given user, newest first.
func (r *OrderRepo) ListByUser(ctx context.Context, userID uuid.UUID, status *string, limit, offset int) ([]models.Order, error) {
	q := `SELECT o.id, o.sale_id, o.code, o.status, o.notes, o.created_at, o.updated_at
		FROM orders o
		JOIN sales s ON s.id = o.sale_id
		JOIN customers c ON c.id = s.customer_id
		WHERE c.user_id = $1`
	args := []interface{}{userID}
	idx := 2
	if status != nil && *status != "" {
		q += " AND o.status = $" + itoa(idx)
		args = append(args, *status)
		idx++
	}
	q += " ORDER BY o.created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
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
