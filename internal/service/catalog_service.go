package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/repository"
)

var ErrInvalidInput = errors.New("invalid input")

type CatalogService struct {
	catalog   *repository.CatalogRepo
	branches  *repository.BranchRepo
	inventory *repository.InventoryRepo
}

func NewCatalogService(c *repository.CatalogRepo, b *repository.BranchRepo, i *repository.InventoryRepo) *CatalogService {
	return &CatalogService{catalog: c, branches: b, inventory: i}
}

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

type CreateVariantInput struct {
	ProductID     uuid.UUID
	SKU           string
	Size          string
	Color         string
	PriceOverride *float64
}

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

func (s *CatalogService) UpsertInventory(ctx context.Context, variantID, branchID uuid.UUID, quantity, reorderLevel int) (*models.Inventory, error) {
	if quantity < 0 || reorderLevel < 0 {
		return nil, ErrInvalidInput
	}
	return s.inventory.Upsert(ctx, variantID, branchID, quantity, reorderLevel)
}
