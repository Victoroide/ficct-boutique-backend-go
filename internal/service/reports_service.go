package service

import (
	"context"

	"github.com/ficct-boutique/backend-go/internal/repository"
)

// ReportsService exposes aggregate reporting queries (sales trends, popular
// products, and the dashboard summary) and clamps caller-supplied ranges to
// sane bounds.
type ReportsService struct {
	reports *repository.ReportsRepo
}

// NewReportsService constructs a ReportsService from the reports repository.
func NewReportsService(r *repository.ReportsRepo) *ReportsService {
	return &ReportsService{reports: r}
}

// MonthlySales returns confirmed-sales totals grouped by month. The months
// window is clamped to (0, 60]; out-of-range values default to 12.
func (s *ReportsService) MonthlySales(ctx context.Context, months int) ([]repository.MonthlySaleRow, error) {
	if months <= 0 || months > 60 {
		months = 12
	}
	return s.reports.MonthlySales(ctx, months)
}

// PopularProducts returns the best-selling products by units sold. The limit
// is clamped to (0, 100]; out-of-range values default to 10.
func (s *ReportsService) PopularProducts(ctx context.Context, limit int) ([]repository.PopularProductRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return s.reports.PopularProducts(ctx, limit)
}

// Dashboard returns the aggregate dashboard summary (today's sales and orders,
// pending orders, low-stock count, and active product/branch counts).
func (s *ReportsService) Dashboard(ctx context.Context) (*repository.DashboardSummary, error) {
	return s.reports.DashboardSummary(ctx)
}
