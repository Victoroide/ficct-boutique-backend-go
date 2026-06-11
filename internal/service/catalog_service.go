package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/repository"
)

var ErrInvalidInput = errors.New("invalid input")

// CatalogService implements the product catalog use cases (products, variants,
// and per-branch inventory), enforcing input validation over the repositories.
type CatalogService struct {
	catalog   *repository.CatalogRepo
	branches  *repository.BranchRepo
	inventory *repository.InventoryRepo
}

// NewCatalogService constructs a CatalogService from the catalog, branch, and
// inventory repositories.
func NewCatalogService(c *repository.CatalogRepo, b *repository.BranchRepo, i *repository.InventoryRepo) *CatalogService {
	return &CatalogService{catalog: c, branches: b, inventory: i}
}

// CreateProductInput carries the fields required to create a catalog product.
type CreateProductInput struct {
	CollectionID    *uuid.UUID
	SKU             string
	Name            string
	Description     *string
	Category        string
	BasePrice       float64
	Currency        string
	ImageURL        *string
	ImageDocumentID *uuid.UUID
}

// CreateProduct validates the input (SKU, name, category required; non-negative
// price; defaulting currency to BOB) and persists a new product.
func (s *CatalogService) CreateProduct(ctx context.Context, in CreateProductInput) (*models.Product, error) {
	if in.SKU == "" || in.Name == "" || in.Category == "" {
		return nil, ErrInvalidInput
	}
	if in.BasePrice < 0 {
		return nil, ErrInvalidInput
	}
	if in.Currency == "" {
		in.Currency = "BOB"
	}
	return s.catalog.CreateProduct(ctx, &models.Product{
		CollectionID:    in.CollectionID,
		SKU:             in.SKU,
		Name:            in.Name,
		Description:     in.Description,
		Category:        in.Category,
		BasePrice:       in.BasePrice,
		Currency:        in.Currency,
		ImageURL:        in.ImageURL,
		ImageDocumentID: in.ImageDocumentID,
	})
}

// CreateVariantInput carries the fields required to create a product variant.
type CreateVariantInput struct {
	ProductID     uuid.UUID
	SKU           string
	Size          string
	Color         string
	PriceOverride *float64
}

// CreateVariant validates the input (SKU, size, and color required) and
// persists a new variant of an existing product.
func (s *CatalogService) CreateVariant(ctx context.Context, in CreateVariantInput) (*models.ProductVariant, error) {
	if in.SKU == "" || in.Size == "" || in.Color == "" {
		return nil, ErrInvalidInput
	}
	return s.catalog.CreateVariant(ctx, &models.ProductVariant{
		ProductID:     in.ProductID,
		SKU:           in.SKU,
		Size:          in.Size,
		Color:         in.Color,
		PriceOverride: in.PriceOverride,
	})
}

// UpsertInventory sets the stock quantity and reorder level for a variant at a
// branch, rejecting negative values.
func (s *CatalogService) UpsertInventory(ctx context.Context, variantID, branchID uuid.UUID, quantity, reorderLevel int) (*models.Inventory, error) {
	if quantity < 0 || reorderLevel < 0 {
		return nil, ErrInvalidInput
	}
	return s.inventory.Upsert(ctx, variantID, branchID, quantity, reorderLevel)
}
