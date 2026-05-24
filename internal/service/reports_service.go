package service

import (
	"context"

	"github.com/ficct-boutique/backend-go/internal/repository"
)

type ReportsService struct {
	reports *repository.ReportsRepo
}

func NewReportsService(r *repository.ReportsRepo) *ReportsService {
	return &ReportsService{reports: r}
}

func (s *ReportsService) MonthlySales(ctx context.Context, months int) ([]repository.MonthlySaleRow, error) {
	if months <= 0 || months > 60 {
		months = 12
	}
	return s.reports.MonthlySales(ctx, months)
}

func (s *ReportsService) PopularProducts(ctx context.Context, limit int) ([]repository.PopularProductRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return s.reports.PopularProducts(ctx, limit)
}

func (s *ReportsService) Dashboard(ctx context.Context) (*repository.DashboardSummary, error) {
	return s.reports.DashboardSummary(ctx)
}
