package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReportsRepo struct {
	pool *pgxpool.Pool
}

func NewReportsRepo(pool *pgxpool.Pool) *ReportsRepo {
	return &ReportsRepo{pool: pool}
}

type MonthlySaleRow struct {
	Month      time.Time
	TotalSales float64
	SaleCount  int
}

func (r *ReportsRepo) MonthlySales(ctx context.Context, months int) ([]MonthlySaleRow, error) {
	const q = `SELECT date_trunc('month', created_at) AS m,
			COALESCE(SUM(total), 0) AS total_sales,
			COUNT(*) AS sale_count
		FROM sales
		WHERE status = 'confirmed' AND created_at >= NOW() - ($1::int || ' months')::interval
		GROUP BY m
		ORDER BY m`
	rows, err := r.pool.Query(ctx, q, months)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MonthlySaleRow{}
	for rows.Next() {
		row := MonthlySaleRow{}
		if err := rows.Scan(&row.Month, &row.TotalSales, &row.SaleCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type PopularProductRow struct {
	ProductID   uuid.UUID
	ProductName string
	UnitsSold   int
	Revenue     float64
}

func (r *ReportsRepo) PopularProducts(ctx context.Context, limit int) ([]PopularProductRow, error) {
	const q = `SELECT p.id, p.name, SUM(si.quantity) AS units, SUM(si.line_total) AS revenue
		FROM sale_items si
		JOIN product_variants v ON v.id = si.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN sales s ON s.id = si.sale_id AND s.status = 'confirmed'
		GROUP BY p.id, p.name
		ORDER BY units DESC
		LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PopularProductRow{}
	for rows.Next() {
		row := PopularProductRow{}
		if err := rows.Scan(&row.ProductID, &row.ProductName, &row.UnitsSold, &row.Revenue); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type DashboardSummary struct {
	TodaySales     float64
	TodayOrders    int
	PendingOrders  int
	LowStockCount  int
	ActiveProducts int
	ActiveBranches int
}

func (r *ReportsRepo) DashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	d := &DashboardSummary{}
	if err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN status='confirmed' AND created_at::date = CURRENT_DATE THEN total ELSE 0 END), 0),
			COUNT(*) FILTER (WHERE status='confirmed' AND created_at::date = CURRENT_DATE)
		FROM sales`).Scan(&d.TodaySales, &d.TodayOrders); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status IN ('placed','preparing')`).Scan(&d.PendingOrders); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM inventory WHERE quantity <= reorder_level`).Scan(&d.LowStockCount); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE is_active = TRUE`).Scan(&d.ActiveProducts); err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM branches WHERE is_active = TRUE`).Scan(&d.ActiveBranches); err != nil {
		return nil, err
	}
	return d, nil
}
