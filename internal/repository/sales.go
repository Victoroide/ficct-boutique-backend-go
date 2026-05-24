package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SalesRepo struct {
	pool *pgxpool.Pool
}

func NewSalesRepo(pool *pgxpool.Pool) *SalesRepo {
	return &SalesRepo{pool: pool}
}

func (r *SalesRepo) Pool() *pgxpool.Pool { return r.pool }

func (r *SalesRepo) CreateWithItems(
	ctx context.Context,
	tx pgx.Tx,
	customerID *uuid.UUID,
	branchID uuid.UUID,
	cashierID *uuid.UUID,
	items []models.SaleItem,
) (*models.Sale, []models.SaleItem, error) {
	sale := &models.Sale{
		Status:   models.SaleStatusPending,
		Currency: "BOB",
	}
	var subtotal float64
	for _, it := range items {
		subtotal += it.LineTotal
	}
	tax := subtotal * 0.13
	total := subtotal + tax
	sale.Subtotal = subtotal
	sale.Tax = tax
	sale.Total = total

	const insertSale = `INSERT INTO sales (customer_id, branch_id, cashier_id, status, subtotal, tax, total, currency)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, customer_id, branch_id, cashier_id, status, subtotal, tax, total, currency, confirmed_at, created_at, updated_at`
	err := tx.QueryRow(ctx, insertSale,
		customerID, branchID, cashierID, sale.Status, sale.Subtotal, sale.Tax, sale.Total, sale.Currency,
	).Scan(
		&sale.ID, &sale.CustomerID, &sale.BranchID, &sale.CashierID, &sale.Status,
		&sale.Subtotal, &sale.Tax, &sale.Total, &sale.Currency, &sale.ConfirmedAt, &sale.CreatedAt, &sale.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}

	out := make([]models.SaleItem, 0, len(items))
	const insertItem = `INSERT INTO sale_items (sale_id, variant_id, quantity, unit_price, line_total)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, sale_id, variant_id, quantity, unit_price, line_total, created_at`
	for _, it := range items {
		row := models.SaleItem{}
		err := tx.QueryRow(ctx, insertItem,
			sale.ID, it.VariantID, it.Quantity, it.UnitPrice, it.LineTotal,
		).Scan(&row.ID, &row.SaleID, &row.VariantID, &row.Quantity, &row.UnitPrice, &row.LineTotal, &row.CreatedAt)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, row)
	}
	return sale, out, nil
}

func (r *SalesRepo) Confirm(ctx context.Context, tx pgx.Tx, saleID uuid.UUID) (*models.Sale, error) {
	const q = `UPDATE sales SET status = 'confirmed', confirmed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
		RETURNING id, customer_id, branch_id, cashier_id, status, subtotal, tax, total, currency, confirmed_at, created_at, updated_at`
	s := &models.Sale{}
	err := tx.QueryRow(ctx, q, saleID).Scan(
		&s.ID, &s.CustomerID, &s.BranchID, &s.CashierID, &s.Status,
		&s.Subtotal, &s.Tax, &s.Total, &s.Currency, &s.ConfirmedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *SalesRepo) Find(ctx context.Context, id uuid.UUID) (*models.Sale, []models.SaleItem, error) {
	const qS = `SELECT id, customer_id, branch_id, cashier_id, status, subtotal, tax, total, currency, confirmed_at, created_at, updated_at
		FROM sales WHERE id = $1`
	s := &models.Sale{}
	if err := r.pool.QueryRow(ctx, qS, id).Scan(
		&s.ID, &s.CustomerID, &s.BranchID, &s.CashierID, &s.Status,
		&s.Subtotal, &s.Tax, &s.Total, &s.Currency, &s.ConfirmedAt, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	const qI = `SELECT id, sale_id, variant_id, quantity, unit_price, line_total, created_at
		FROM sale_items WHERE sale_id = $1 ORDER BY created_at`
	rows, err := r.pool.Query(ctx, qI, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := []models.SaleItem{}
	for rows.Next() {
		it := models.SaleItem{}
		if err := rows.Scan(&it.ID, &it.SaleID, &it.VariantID, &it.Quantity, &it.UnitPrice, &it.LineTotal, &it.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, it)
	}
	return s, items, rows.Err()
}

func (r *SalesRepo) List(ctx context.Context, status *string, since, until *time.Time, limit, offset int) ([]models.Sale, error) {
	q := `SELECT id, customer_id, branch_id, cashier_id, status, subtotal, tax, total, currency, confirmed_at, created_at, updated_at
		FROM sales WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if status != nil && *status != "" {
		q += " AND status = $" + itoa(idx)
		args = append(args, *status)
		idx++
	}
	if since != nil {
		q += " AND created_at >= $" + itoa(idx)
		args = append(args, *since)
		idx++
	}
	if until != nil {
		q += " AND created_at <= $" + itoa(idx)
		args = append(args, *until)
		idx++
	}
	q += " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Sale{}
	for rows.Next() {
		s := models.Sale{}
		if err := rows.Scan(&s.ID, &s.CustomerID, &s.BranchID, &s.CashierID, &s.Status,
			&s.Subtotal, &s.Tax, &s.Total, &s.Currency, &s.ConfirmedAt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
