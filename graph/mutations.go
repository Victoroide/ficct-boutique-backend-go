package graph

import (
	"context"
	"errors"

	"github.com/ficct-boutique/backend-go/internal/middleware"
	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/service"
	"github.com/google/uuid"
)

func (r *Resolver) Login(ctx context.Context, args struct {
	Input struct {
		Email    string
		Password string
	}
}) (*AuthPayloadResolver, error) {
	res, err := r.AuthSvc.Login(ctx, args.Input.Email, args.Input.Password)
	if err != nil {
		return nil, err
	}
	return &AuthPayloadResolver{
		token: res.AccessToken,
		exp:   TimeFrom(res.ExpiresAt),
		user:  &UserResolver{M: res.User},
	}, nil
}

func (r *Resolver) CreateCollection(ctx context.Context, args struct {
	Input struct {
		Name        string
		Description *string
		Season      *string
	}
}) (*CollectionResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	c, err := r.CatalogRepo.CreateCollection(ctx, args.Input.Name, args.Input.Description, args.Input.Season)
	if err != nil {
		return nil, err
	}
	return &CollectionResolver{M: c}, nil
}

func (r *Resolver) CreateProduct(ctx context.Context, args struct {
	Input struct {
		CollectionID    *UUID
		SKU             string
		Name            string
		Description     *string
		Category        string
		BasePrice       float64
		Currency        *string
		ImageURL        *string
		ImageDocumentID *UUID
	}
}) (*ProductResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	in := service.CreateProductInput{
		SKU:         args.Input.SKU,
		Name:        args.Input.Name,
		Description: args.Input.Description,
		Category:    args.Input.Category,
		BasePrice:   args.Input.BasePrice,
		ImageURL:    args.Input.ImageURL,
	}
	if args.Input.Currency != nil {
		in.Currency = *args.Input.Currency
	}
	if args.Input.CollectionID != nil {
		v := args.Input.CollectionID.Native()
		in.CollectionID = &v
	}
	if args.Input.ImageDocumentID != nil {
		v := args.Input.ImageDocumentID.Native()
		in.ImageDocumentID = &v
	}
	p, err := r.CatalogSvc.CreateProduct(ctx, in)
	if err != nil {
		return nil, err
	}
	return &ProductResolver{M: p, R: r}, nil
}

func (r *Resolver) UpdateProduct(ctx context.Context, args struct {
	Input struct {
		ID              UUID
		Name            string
		Description     *string
		Category        string
		BasePrice       float64
		ImageURL        *string
		ImageDocumentID *UUID
		IsActive        bool
	}
}) (*ProductResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var imgDoc *uuid.UUID
	if args.Input.ImageDocumentID != nil {
		v := args.Input.ImageDocumentID.Native()
		imgDoc = &v
	}
	p, err := r.CatalogRepo.UpdateProduct(ctx, args.Input.ID.Native(), args.Input.Name, args.Input.Description, args.Input.Category, args.Input.BasePrice, args.Input.ImageURL, imgDoc, args.Input.IsActive)
	if err != nil {
		return nil, err
	}
	return &ProductResolver{M: p, R: r}, nil
}

func (r *Resolver) CreateVariant(ctx context.Context, args struct {
	Input struct {
		ProductID     UUID
		SKU           string
		Size          string
		Color         string
		PriceOverride *float64
	}
}) (*VariantResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	v, err := r.CatalogSvc.CreateVariant(ctx, service.CreateVariantInput{
		ProductID:     args.Input.ProductID.Native(),
		SKU:           args.Input.SKU,
		Size:          args.Input.Size,
		Color:         args.Input.Color,
		PriceOverride: args.Input.PriceOverride,
	})
	if err != nil {
		return nil, err
	}
	return &VariantResolver{M: v, R: r}, nil
}

func (r *Resolver) UpsertInventory(ctx context.Context, args struct {
	Input struct {
		VariantID    UUID
		BranchID     UUID
		Quantity     int32
		ReorderLevel int32
	}
}) (*InventoryEntryResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	inv, err := r.CatalogSvc.UpsertInventory(ctx, args.Input.VariantID.Native(), args.Input.BranchID.Native(), int(args.Input.Quantity), int(args.Input.ReorderLevel))
	if err != nil {
		return nil, err
	}
	return &InventoryEntryResolver{M: inv, R: r}, nil
}

func (r *Resolver) CreateBranch(ctx context.Context, args struct {
	Input struct {
		Code      string
		Name      string
		Address   string
		Latitude  *float64
		Longitude *float64
		Phone     *string
	}
}) (*BranchResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	b, err := r.BranchRepo.Create(ctx, &models.Branch{
		Code:      args.Input.Code,
		Name:      args.Input.Name,
		Address:   args.Input.Address,
		Latitude:  args.Input.Latitude,
		Longitude: args.Input.Longitude,
		Phone:     args.Input.Phone,
	})
	if err != nil {
		return nil, err
	}
	return &BranchResolver{M: b}, nil
}

func (r *Resolver) CreateSale(ctx context.Context, args struct {
	Input struct {
		CustomerID *UUID
		BranchID   UUID
		Items      []struct {
			VariantID UUID
			Quantity  int32
		}
	}
}) (*SaleResolver, error) {
	if err := requireAuth(ctx); err != nil {
		return nil, err
	}
	items := make([]service.CreateSaleItemInput, 0, len(args.Input.Items))
	for _, it := range args.Input.Items {
		items = append(items, service.CreateSaleItemInput{
			VariantID: it.VariantID.Native(),
			Quantity:  int(it.Quantity),
		})
	}

	in := service.CreateSaleInput{
		BranchID: args.Input.BranchID.Native(),
		Items:    items,
	}
	if args.Input.CustomerID != nil {
		v := args.Input.CustomerID.Native()
		in.CustomerID = &v
	}
	// cashier from claims (if staff/admin)
	claims, _ := middleware.ClaimsFromContext(ctx)
	if claims != nil {
		uid, err := parseUUIDFromClaim(claims.Subject)
		if err == nil {
			in.CashierID = &uid
		}
	}

	sale, items2, err := r.SalesSvc.CreateSale(ctx, in)
	if err != nil {
		return nil, err
	}
	return &SaleResolver{M: sale, ItemsCache: items2, R: r}, nil
}

func (r *Resolver) DeactivateProduct(ctx context.Context, args struct{ ID UUID }) (*ProductResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	p, err := r.CatalogRepo.SetProductActive(ctx, args.ID.Native(), false)
	if err != nil {
		return nil, err
	}
	return &ProductResolver{M: p, R: r}, nil
}

func (r *Resolver) ActivateProduct(ctx context.Context, args struct{ ID UUID }) (*ProductResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	p, err := r.CatalogRepo.SetProductActive(ctx, args.ID.Native(), true)
	if err != nil {
		return nil, err
	}
	return &ProductResolver{M: p, R: r}, nil
}

func (r *Resolver) ReplaceProductImage(ctx context.Context, args struct {
	ID                 UUID
	NewImageDocumentID UUID
}) (*ProductResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	p, _, err := r.CatalogRepo.ReplaceProductImage(ctx, args.ID.Native(), args.NewImageDocumentID.Native())
	if err != nil {
		return nil, err
	}
	return &ProductResolver{M: p, R: r}, nil
}

func (r *Resolver) DeactivateVariant(ctx context.Context, args struct{ ID UUID }) (*VariantResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	v, err := r.CatalogRepo.SetVariantActive(ctx, args.ID.Native(), false)
	if err != nil {
		return nil, err
	}
	return &VariantResolver{M: v, R: r}, nil
}

func (r *Resolver) ActivateVariant(ctx context.Context, args struct{ ID UUID }) (*VariantResolver, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	v, err := r.CatalogRepo.SetVariantActive(ctx, args.ID.Native(), true)
	if err != nil {
		return nil, err
	}
	return &VariantResolver{M: v, R: r}, nil
}

func (r *Resolver) SetInventoryStock(ctx context.Context, args struct {
	VariantID UUID
	BranchID  UUID
	Quantity  int32
}) (*InventoryEntryResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	if args.Quantity < 0 {
		return nil, errors.New("quantity must be >= 0")
	}
	inv, err := r.InvRepo.SetStock(ctx, args.VariantID.Native(), args.BranchID.Native(), int(args.Quantity))
	if err != nil {
		return nil, err
	}
	return &InventoryEntryResolver{M: inv, R: r}, nil
}

func (r *Resolver) AdjustInventoryStock(ctx context.Context, args struct {
	VariantID UUID
	BranchID  UUID
	Delta     int32
}) (*InventoryEntryResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	inv, err := r.InvRepo.AdjustStock(ctx, args.VariantID.Native(), args.BranchID.Native(), int(args.Delta))
	if err != nil {
		return nil, err
	}
	return &InventoryEntryResolver{M: inv, R: r}, nil
}

func (r *Resolver) UpdateInventoryReorderLevel(ctx context.Context, args struct {
	VariantID    UUID
	BranchID     UUID
	ReorderLevel int32
}) (*InventoryEntryResolver, error) {
	if err := requireAdminOrStaff(ctx); err != nil {
		return nil, err
	}
	if args.ReorderLevel < 0 {
		return nil, errors.New("reorderLevel must be >= 0")
	}
	inv, err := r.InvRepo.SetReorderLevel(ctx, args.VariantID.Native(), args.BranchID.Native(), int(args.ReorderLevel))
	if err != nil {
		return nil, err
	}
	return &InventoryEntryResolver{M: inv, R: r}, nil
}

func (r *Resolver) ConfirmSale(ctx context.Context, args struct{ SaleID UUID }) (*OrderResolver, error) {
	if err := requireAuth(ctx); err != nil {
		return nil, err
	}
	res, err := r.SalesSvc.ConfirmSale(ctx, args.SaleID.Native())
	if err != nil {
		if errors.Is(err, service.ErrSaleNotPending) {
			return nil, errors.New("sale is not in pending status")
		}
		return nil, err
	}
	return &OrderResolver{M: res.Order, R: r}, nil
}
