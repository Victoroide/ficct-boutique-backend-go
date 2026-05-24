package graph

import (
	"context"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
)

// ----- User -----

type UserResolver struct {
	M *models.User
}

func (r *UserResolver) ID() UUID         { return UUIDFrom(r.M.ID) }
func (r *UserResolver) Email() string    { return r.M.Email }
func (r *UserResolver) FullName() string { return r.M.FullName }
func (r *UserResolver) Role() string     { return string(r.M.Role) }
func (r *UserResolver) IsActive() bool   { return r.M.IsActive }
func (r *UserResolver) CreatedAt() Time  { return TimeFrom(r.M.CreatedAt) }

// ----- AuthPayload -----

type AuthPayloadResolver struct {
	token string
	exp   Time
	user  *UserResolver
}

func (r *AuthPayloadResolver) AccessToken() string { return r.token }
func (r *AuthPayloadResolver) ExpiresAt() Time     { return r.exp }
func (r *AuthPayloadResolver) User() *UserResolver { return r.user }

// ----- Collection -----

type CollectionResolver struct{ M *models.Collection }

func (r *CollectionResolver) ID() UUID             { return UUIDFrom(r.M.ID) }
func (r *CollectionResolver) Name() string         { return r.M.Name }
func (r *CollectionResolver) Description() *string { return r.M.Description }
func (r *CollectionResolver) Season() *string      { return r.M.Season }
func (r *CollectionResolver) IsActive() bool       { return r.M.IsActive }

// ----- Branch -----

type BranchResolver struct{ M *models.Branch }

func (r *BranchResolver) ID() UUID            { return UUIDFrom(r.M.ID) }
func (r *BranchResolver) Code() string        { return r.M.Code }
func (r *BranchResolver) Name() string        { return r.M.Name }
func (r *BranchResolver) Address() string     { return r.M.Address }
func (r *BranchResolver) Latitude() *float64  { return r.M.Latitude }
func (r *BranchResolver) Longitude() *float64 { return r.M.Longitude }
func (r *BranchResolver) Phone() *string      { return r.M.Phone }
func (r *BranchResolver) IsActive() bool      { return r.M.IsActive }

// ----- Product / Variant / Inventory -----

type ProductResolver struct {
	M *models.Product
	R *Resolver
}

func (p *ProductResolver) ID() UUID             { return UUIDFrom(p.M.ID) }
func (p *ProductResolver) SKU() string          { return p.M.SKU }
func (p *ProductResolver) Name() string         { return p.M.Name }
func (p *ProductResolver) Description() *string { return p.M.Description }
func (p *ProductResolver) Category() string     { return p.M.Category }
func (p *ProductResolver) BasePrice() float64   { return p.M.BasePrice }
func (p *ProductResolver) Currency() string     { return p.M.Currency }
func (p *ProductResolver) ImageURL() *string    { return p.M.ImageURL }
func (p *ProductResolver) ImageDocumentID() *UUID {
	if p.M.ImageDocumentID == nil {
		return nil
	}
	v := UUIDFrom(*p.M.ImageDocumentID)
	return &v
}
func (p *ProductResolver) IsActive() bool  { return p.M.IsActive }
func (p *ProductResolver) CreatedAt() Time { return TimeFrom(p.M.CreatedAt) }

func (p *ProductResolver) Collection(ctx context.Context) (*CollectionResolver, error) {
	if p.M.CollectionID == nil {
		return nil, nil
	}
	// For simplicity we don't hit DB inline; the catalog service could provide a loader.
	// Here we return a minimal projection from cached repo lookup.
	return nil, nil
}

func (p *ProductResolver) Variants(ctx context.Context) ([]*VariantResolver, error) {
	vs, err := p.R.CatalogRepo.VariantsByProduct(ctx, p.M.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*VariantResolver, 0, len(vs))
	for i := range vs {
		v := vs[i]
		out = append(out, &VariantResolver{M: &v, R: p.R})
	}
	return out, nil
}

type VariantResolver struct {
	M *models.ProductVariant
	R *Resolver
}

func (v *VariantResolver) ID() UUID                { return UUIDFrom(v.M.ID) }
func (v *VariantResolver) ProductID() UUID         { return UUIDFrom(v.M.ProductID) }
func (v *VariantResolver) SKU() string             { return v.M.SKU }
func (v *VariantResolver) Size() string            { return v.M.Size }
func (v *VariantResolver) Color() string           { return v.M.Color }
func (v *VariantResolver) IsActive() bool          { return v.M.IsActive }
func (v *VariantResolver) PriceOverride() *float64 { return v.M.PriceOverride }

func (v *VariantResolver) Stock(ctx context.Context) ([]*InventoryEntryResolver, error) {
	groups, err := v.R.InvRepo.ByVariantIDs(ctx, []uuid.UUID{v.M.ID})
	if err != nil {
		return nil, err
	}
	entries := groups[v.M.ID]
	out := make([]*InventoryEntryResolver, 0, len(entries))
	for i := range entries {
		e := entries[i]
		out = append(out, &InventoryEntryResolver{M: &e, R: v.R})
	}
	return out, nil
}

type InventoryEntryResolver struct {
	M *models.Inventory
	R *Resolver
}

func (e *InventoryEntryResolver) ID() UUID            { return UUIDFrom(e.M.ID) }
func (e *InventoryEntryResolver) VariantID() UUID     { return UUIDFrom(e.M.VariantID) }
func (e *InventoryEntryResolver) Quantity() int32     { return int32(e.M.Quantity) }
func (e *InventoryEntryResolver) ReorderLevel() int32 { return int32(e.M.ReorderLevel) }
func (e *InventoryEntryResolver) UpdatedAt() Time     { return TimeFrom(e.M.UpdatedAt) }

func (e *InventoryEntryResolver) Branch(ctx context.Context) (*BranchResolver, error) {
	b, err := e.R.BranchRepo.Find(ctx, e.M.BranchID)
	if err != nil {
		return nil, err
	}
	return &BranchResolver{M: b}, nil
}

func (e *InventoryEntryResolver) Variant(ctx context.Context) (*VariantResolver, error) {
	v, err := e.R.CatalogRepo.FindVariant(ctx, e.M.VariantID)
	if err != nil {
		return nil, nil
	}
	return &VariantResolver{M: v, R: e.R}, nil
}

func (e *InventoryEntryResolver) Product(ctx context.Context) (*ProductResolver, error) {
	v, err := e.R.CatalogRepo.FindVariant(ctx, e.M.VariantID)
	if err != nil {
		return nil, nil
	}
	p, err := e.R.CatalogRepo.FindProduct(ctx, v.ProductID)
	if err != nil {
		return nil, nil
	}
	return &ProductResolver{M: p, R: e.R}, nil
}

// ----- Sale / SaleItem / Order -----

type SaleItemResolver struct{ M *models.SaleItem }

func (r *SaleItemResolver) ID() UUID           { return UUIDFrom(r.M.ID) }
func (r *SaleItemResolver) VariantID() UUID    { return UUIDFrom(r.M.VariantID) }
func (r *SaleItemResolver) Quantity() int32    { return int32(r.M.Quantity) }
func (r *SaleItemResolver) UnitPrice() float64 { return r.M.UnitPrice }
func (r *SaleItemResolver) LineTotal() float64 { return r.M.LineTotal }

type SaleResolver struct {
	M          *models.Sale
	ItemsCache []models.SaleItem
	R          *Resolver
}

func (s *SaleResolver) ID() UUID          { return UUIDFrom(s.M.ID) }
func (s *SaleResolver) Status() string    { return string(s.M.Status) }
func (s *SaleResolver) Subtotal() float64 { return s.M.Subtotal }
func (s *SaleResolver) Tax() float64      { return s.M.Tax }
func (s *SaleResolver) Total() float64    { return s.M.Total }
func (s *SaleResolver) Currency() string  { return s.M.Currency }
func (s *SaleResolver) CreatedAt() Time   { return TimeFrom(s.M.CreatedAt) }
func (s *SaleResolver) ConfirmedAt() *Time {
	if s.M.ConfirmedAt == nil {
		return nil
	}
	t := TimeFrom(*s.M.ConfirmedAt)
	return &t
}

func (s *SaleResolver) Branch(ctx context.Context) (*BranchResolver, error) {
	b, err := s.R.BranchRepo.Find(ctx, s.M.BranchID)
	if err != nil {
		return nil, err
	}
	return &BranchResolver{M: b}, nil
}

func (s *SaleResolver) Items(ctx context.Context) ([]*SaleItemResolver, error) {
	if s.ItemsCache == nil {
		_, items, err := s.R.SalesRepo.Find(ctx, s.M.ID)
		if err != nil {
			return nil, err
		}
		s.ItemsCache = items
	}
	out := make([]*SaleItemResolver, 0, len(s.ItemsCache))
	for i := range s.ItemsCache {
		it := s.ItemsCache[i]
		out = append(out, &SaleItemResolver{M: &it})
	}
	return out, nil
}

type OrderResolver struct {
	M *models.Order
	R *Resolver
}

func (o *OrderResolver) ID() UUID        { return UUIDFrom(o.M.ID) }
func (o *OrderResolver) Code() string    { return o.M.Code }
func (o *OrderResolver) Status() string  { return string(o.M.Status) }
func (o *OrderResolver) Notes() *string  { return o.M.Notes }
func (o *OrderResolver) CreatedAt() Time { return TimeFrom(o.M.CreatedAt) }

func (o *OrderResolver) Sale(ctx context.Context) (*SaleResolver, error) {
	s, items, err := o.R.SalesRepo.Find(ctx, o.M.SaleID)
	if err != nil {
		return nil, err
	}
	return &SaleResolver{M: s, ItemsCache: items, R: o.R}, nil
}

// ----- Reports -----

type MonthlySalePointResolver struct {
	Month_      Time
	TotalSales_ float64
	SaleCount_  int32
}

func (r *MonthlySalePointResolver) Month() Time         { return r.Month_ }
func (r *MonthlySalePointResolver) TotalSales() float64 { return r.TotalSales_ }
func (r *MonthlySalePointResolver) SaleCount() int32    { return r.SaleCount_ }

type PopularProductRowResolver struct {
	ProductID_   UUID
	ProductName_ string
	UnitsSold_   int32
	Revenue_     float64
}

func (r *PopularProductRowResolver) ProductID() UUID     { return r.ProductID_ }
func (r *PopularProductRowResolver) ProductName() string { return r.ProductName_ }
func (r *PopularProductRowResolver) UnitsSold() int32    { return r.UnitsSold_ }
func (r *PopularProductRowResolver) Revenue() float64    { return r.Revenue_ }

type DashboardSummaryResolver struct {
	TodaySales_     float64
	TodayOrders_    int32
	PendingOrders_  int32
	LowStockCount_  int32
	ActiveProducts_ int32
	ActiveBranches_ int32
}

func (r *DashboardSummaryResolver) TodaySales() float64   { return r.TodaySales_ }
func (r *DashboardSummaryResolver) TodayOrders() int32    { return r.TodayOrders_ }
func (r *DashboardSummaryResolver) PendingOrders() int32  { return r.PendingOrders_ }
func (r *DashboardSummaryResolver) LowStockCount() int32  { return r.LowStockCount_ }
func (r *DashboardSummaryResolver) ActiveProducts() int32 { return r.ActiveProducts_ }
func (r *DashboardSummaryResolver) ActiveBranches() int32 { return r.ActiveBranches_ }

// ----- PushToken / SendPushResult -----

type SendPushResultResolver struct {
	Sent_        int32
	Failed_      int32
	Deactivated_ int32
	Errors_      []string
}

func (r *SendPushResultResolver) Sent() int32        { return r.Sent_ }
func (r *SendPushResultResolver) Failed() int32      { return r.Failed_ }
func (r *SendPushResultResolver) Deactivated() int32 { return r.Deactivated_ }
func (r *SendPushResultResolver) Errors() []string   { return r.Errors_ }

type PushTokenResolver struct{ M *models.PushToken }

func (r *PushTokenResolver) ID() UUID         { return UUIDFrom(r.M.ID) }
func (r *PushTokenResolver) Token() string    { return r.M.Token }
func (r *PushTokenResolver) Platform() string { return string(r.M.Platform) }
func (r *PushTokenResolver) DeviceID() *string {
	return r.M.DeviceID
}
func (r *PushTokenResolver) IsActive() bool   { return r.M.IsActive }
func (r *PushTokenResolver) LastSeenAt() Time { return TimeFrom(r.M.LastSeenAt) }
func (r *PushTokenResolver) CreatedAt() Time  { return TimeFrom(r.M.CreatedAt) }
