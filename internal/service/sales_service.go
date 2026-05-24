package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrEmptySaleItems = errors.New("sale must contain at least one item")
	ErrSaleNotPending = errors.New("sale is not pending")
)

type SalesService struct {
	sales            *repository.SalesRepo
	inventory        *repository.InventoryRepo
	orders           *repository.OrderRepo
	catalog          *repository.CatalogRepo
	outbox           *repository.OutboxRepo
	webhookTargetURL string
}

type CreateSaleInput struct {
	CustomerID *uuid.UUID
	BranchID   uuid.UUID
	CashierID  *uuid.UUID
	Items      []CreateSaleItemInput
}

type CreateSaleItemInput struct {
	VariantID uuid.UUID
	Quantity  int
}

type SalesServiceDeps struct {
	Sales            *repository.SalesRepo
	Inventory        *repository.InventoryRepo
	Orders           *repository.OrderRepo
	Catalog          *repository.CatalogRepo
	Outbox           *repository.OutboxRepo
	WebhookTargetURL string
}

func NewSalesService(d SalesServiceDeps) *SalesService {
	return &SalesService{
		sales:            d.Sales,
		inventory:        d.Inventory,
		orders:           d.Orders,
		catalog:          d.Catalog,
		outbox:           d.Outbox,
		webhookTargetURL: d.WebhookTargetURL,
	}
}

func (s *SalesService) CreateSale(ctx context.Context, in CreateSaleInput) (*models.Sale, []models.SaleItem, error) {
	if len(in.Items) == 0 {
		return nil, nil, ErrEmptySaleItems
	}

	items := make([]models.SaleItem, 0, len(in.Items))
	for _, it := range in.Items {
		v, err := s.catalog.FindVariant(ctx, it.VariantID)
		if err != nil {
			return nil, nil, fmt.Errorf("variant %s: %w", it.VariantID, err)
		}
		p, err := s.catalog.FindProduct(ctx, v.ProductID)
		if err != nil {
			return nil, nil, fmt.Errorf("product for variant %s: %w", it.VariantID, err)
		}
		unitPrice := p.BasePrice
		if v.PriceOverride != nil {
			unitPrice = *v.PriceOverride
		}
		items = append(items, models.SaleItem{
			VariantID: it.VariantID,
			Quantity:  it.Quantity,
			UnitPrice: unitPrice,
			LineTotal: unitPrice * float64(it.Quantity),
		})
	}

	tx, err := s.sales.Pool().Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	sale, savedItems, err := s.sales.CreateWithItems(ctx, tx, in.CustomerID, in.BranchID, in.CashierID, items)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return sale, savedItems, nil
}

type ConfirmSaleResult struct {
	Sale  *models.Sale
	Order *models.Order
}

func (s *SalesService) ConfirmSale(ctx context.Context, saleID uuid.UUID) (*ConfirmSaleResult, error) {
	tx, err := s.sales.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	sale, items, err := s.sales.Find(ctx, saleID)
	if err != nil {
		return nil, err
	}
	if sale.Status != models.SaleStatusPending {
		return nil, ErrSaleNotPending
	}

	for _, it := range items {
		if err := s.inventory.DecrementForSale(ctx, tx, it.VariantID, sale.BranchID, it.Quantity); err != nil {
			return nil, fmt.Errorf("decrement stock for variant %s: %w", it.VariantID, err)
		}
	}

	confirmed, err := s.sales.Confirm(ctx, tx, saleID)
	if err != nil {
		return nil, err
	}

	code := fmt.Sprintf("ORD-%s-%04d", time.Now().UTC().Format("20060102"), uuid.New().ID()%10000)
	order, err := s.orders.CreateForSale(ctx, tx, confirmed.ID, code)
	if err != nil {
		return nil, err
	}

	if s.webhookTargetURL != "" {
		payload := map[string]interface{}{
			"event":        "sale.confirmed",
			"sale_id":      confirmed.ID,
			"order_id":     order.ID,
			"order_code":   order.Code,
			"branch_id":    confirmed.BranchID,
			"total":        confirmed.Total,
			"currency":     confirmed.Currency,
			"confirmed_at": confirmed.ConfirmedAt,
			"items":        items,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		if _, err := s.outbox.Enqueue(ctx, tx, "sale.confirmed", confirmed.ID, raw, s.webhookTargetURL); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ConfirmSaleResult{Sale: confirmed, Order: order}, nil
}
