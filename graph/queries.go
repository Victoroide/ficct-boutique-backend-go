package graph

import (
	"context"
	"errors"

	"github.com/ficct-boutique/backend-go/internal/middleware"
	"github.com/ficct-boutique/backend-go/internal/repository"
	"github.com/google/uuid"
)

func (r *Resolver) Me(ctx context.Context) (*UserResolver, error) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return nil, nil
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, err
	}
	u, err := r.UserRepo.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &UserResolver{M: u}, nil
}

func (r *Resolver) Product(ctx context.Context, args struct{ ID UUID }) (*ProductResolver, error) {
	p, err := r.CatalogRepo.FindProduct(ctx, args.ID.Native())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ProductResolver{M: p, R: r}, nil
}

func (r *Resolver) Products(ctx context.Context, args struct {
	Category        *string
	Search          *string
	IncludeInactive *bool
	Limit           *int32
	Offset          *int32
}) ([]*ProductResolver, error) {
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	offset := 0
	if args.Offset != nil {
		offset = int(*args.Offset)
	}
	includeInactive := false
	if args.IncludeInactive != nil && *args.IncludeInactive {
		// Only admin/staff can ask for inactive products. Customers always get active-only.
		if err := requireAdminOrStaff(ctx); err == nil {
			includeInactive = true
		}
	}
	ps, err := r.CatalogRepo.ListProducts(ctx, args.Category, args.Search, includeInactive, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*ProductResolver, 0, len(ps))
	for i := range ps {
		p := ps[i]
		out = append(out, &ProductResolver{M: &p, R: r})
	}
	return out, nil
}

func (r *Resolver) Branches(ctx context.Context) ([]*BranchResolver, error) {
	bs, err := r.BranchRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*BranchResolver, 0, len(bs))
	for i := range bs {
		b := bs[i]
		out = append(out, &BranchResolver{M: &b})
	}
	return out, nil
}

func (r *Resolver) Branch(ctx context.Context, args struct{ ID UUID }) (*BranchResolver, error) {
	b, err := r.BranchRepo.Find(ctx, args.ID.Native())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &BranchResolver{M: b}, nil
}

func (r *Resolver) InventoryByBranch(ctx context.Context, args struct{ BranchID UUID }) ([]*InventoryEntryResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	rows, err := r.InvRepo.ByBranch(ctx, args.BranchID.Native())
	if err != nil {
		return nil, err
	}
	out := make([]*InventoryEntryResolver, 0, len(rows))
	for i := range rows {
		row := rows[i]
		out = append(out, &InventoryEntryResolver{M: &row, R: r})
	}
	return out, nil
}

type InventoryPageResolver struct {
	EntriesData []*InventoryEntryResolver
	TotalData   int32
	LimitData   int32
	OffsetData  int32
}

func (p *InventoryPageResolver) Entries() []*InventoryEntryResolver { return p.EntriesData }
func (p *InventoryPageResolver) Total() int32                       { return p.TotalData }
func (p *InventoryPageResolver) Limit() int32                       { return p.LimitData }
func (p *InventoryPageResolver) Offset() int32                      { return p.OffsetData }

func (r *Resolver) InventoryEntries(ctx context.Context, args struct {
	Filter *struct {
		BranchID                *UUID
		Search                  *string
		Size                    *string
		Color                   *string
		Status                  *string
		OnlyLowStock            *bool
		IncludeInactiveVariants *bool
	}
	Limit  *int32
	Offset *int32
}) (*InventoryPageResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	limit := 25
	if args.Limit != nil {
		l := int(*args.Limit)
		if l >= 1 && l <= 200 {
			limit = l
		}
	}
	offset := 0
	if args.Offset != nil && *args.Offset >= 0 {
		offset = int(*args.Offset)
	}

	f := repository.InventoryFilter{}
	if args.Filter != nil {
		if args.Filter.BranchID != nil {
			b := args.Filter.BranchID.Native()
			f.BranchID = &b
		}
		f.Search = args.Filter.Search
		f.Size = args.Filter.Size
		f.Color = args.Filter.Color
		f.Status = args.Filter.Status
		if args.Filter.OnlyLowStock != nil {
			f.OnlyLowStock = *args.Filter.OnlyLowStock
		}
		if args.Filter.IncludeInactiveVariants != nil {
			f.IncludeInactiveVariants = *args.Filter.IncludeInactiveVariants
		}
	}

	rows, total, err := r.InvRepo.SearchInventory(ctx, f, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*InventoryEntryResolver, 0, len(rows))
	for i := range rows {
		row := rows[i]
		out = append(out, &InventoryEntryResolver{M: &row, R: r})
	}
	return &InventoryPageResolver{
		EntriesData: out,
		TotalData:   int32(total),
		LimitData:   int32(limit),
		OffsetData:  int32(offset),
	}, nil
}

func (r *Resolver) Sale(ctx context.Context, args struct{ ID UUID }) (*SaleResolver, error) {
	if err := requireAuth(ctx); err != nil {
		return nil, err
	}
	s, items, err := r.SalesRepo.Find(ctx, args.ID.Native())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &SaleResolver{M: s, ItemsCache: items, R: r}, nil
}

func (r *Resolver) Sales(ctx context.Context, args struct {
	Status *string
	Limit  *int32
	Offset *int32
}) ([]*SaleResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	offset := 0
	if args.Offset != nil {
		offset = int(*args.Offset)
	}
	rows, err := r.SalesRepo.List(ctx, args.Status, nil, nil, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*SaleResolver, 0, len(rows))
	for i := range rows {
		row := rows[i]
		out = append(out, &SaleResolver{M: &row, R: r})
	}
	return out, nil
}

func (r *Resolver) Order(ctx context.Context, args struct{ ID UUID }) (*OrderResolver, error) {
	if err := requireAuth(ctx); err != nil {
		return nil, err
	}
	o, err := r.OrderRepo.Find(ctx, args.ID.Native())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &OrderResolver{M: o, R: r}, nil
}

func (r *Resolver) Orders(ctx context.Context, args struct {
	Status *string
	Limit  *int32
	Offset *int32
}) ([]*OrderResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	limit := 50
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	offset := 0
	if args.Offset != nil {
		offset = int(*args.Offset)
	}
	rows, err := r.OrderRepo.List(ctx, args.Status, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*OrderResolver, 0, len(rows))
	for i := range rows {
		row := rows[i]
		out = append(out, &OrderResolver{M: &row, R: r})
	}
	return out, nil
}

func (r *Resolver) MonthlySales(ctx context.Context, args struct{ Months *int32 }) ([]*MonthlySalePointResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	months := 12
	if args.Months != nil {
		months = int(*args.Months)
	}
	rows, err := r.ReportsSvc.MonthlySales(ctx, months)
	if err != nil {
		return nil, err
	}
	out := make([]*MonthlySalePointResolver, 0, len(rows))
	for _, row := range rows {
		out = append(out, &MonthlySalePointResolver{
			Month_:      TimeFrom(row.Month),
			TotalSales_: row.TotalSales,
			SaleCount_:  int32(row.SaleCount),
		})
	}
	return out, nil
}

func (r *Resolver) PopularProducts(ctx context.Context, args struct{ Limit *int32 }) ([]*PopularProductRowResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	limit := 10
	if args.Limit != nil {
		limit = int(*args.Limit)
	}
	rows, err := r.ReportsSvc.PopularProducts(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*PopularProductRowResolver, 0, len(rows))
	for _, row := range rows {
		out = append(out, &PopularProductRowResolver{
			ProductID_:   UUIDFrom(row.ProductID),
			ProductName_: row.ProductName,
			UnitsSold_:   int32(row.UnitsSold),
			Revenue_:     row.Revenue,
		})
	}
	return out, nil
}

func (r *Resolver) DashboardSummary(ctx context.Context) (*DashboardSummaryResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	d, err := r.ReportsSvc.Dashboard(ctx)
	if err != nil {
		return nil, err
	}
	return &DashboardSummaryResolver{
		TodaySales_:     d.TodaySales,
		TodayOrders_:    int32(d.TodayOrders),
		PendingOrders_:  int32(d.PendingOrders),
		LowStockCount_:  int32(d.LowStockCount),
		ActiveProducts_: int32(d.ActiveProducts),
		ActiveBranches_: int32(d.ActiveBranches),
	}, nil
}
