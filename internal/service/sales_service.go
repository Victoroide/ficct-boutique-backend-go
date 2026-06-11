package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/repository"
)

var (
	ErrEmptySaleItems = errors.New("sale must contain at least one item")
	ErrSaleNotPending = errors.New("sale is not pending")
)

// SalesService implements the point-of-sale use cases: building draft sales
// from current catalog prices and confirming them, which decrements stock,
// creates an order, and (when configured) enqueues a webhook in the outbox,
// all within a single database transaction.
type SalesService struct {
	sales            *repository.SalesRepo
	inventory        *repository.InventoryRepo
	orders           *repository.OrderRepo
	catalog          *repository.CatalogRepo
	outbox           *repository.OutboxRepo
	webhookTargetURL string
}

// CreateSaleInput describes a new sale: optional customer and cashier, the
// branch it occurs at, and the line items being purchased.
type CreateSaleInput struct {
	CustomerID *uuid.UUID
	BranchID   uuid.UUID
	CashierID  *uuid.UUID
	Items      []CreateSaleItemInput
}

// CreateSaleItemInput is a single requested line item: a variant and quantity.
type CreateSaleItemInput struct {
	VariantID uuid.UUID
	Quantity  int
}

// SalesServiceDeps bundles the repositories and configuration needed to build a
// SalesService.
type SalesServiceDeps struct {
	Sales            *repository.SalesRepo
	Inventory        *repository.InventoryRepo
	Orders           *repository.OrderRepo
	Catalog          *repository.CatalogRepo
	Outbox           *repository.OutboxRepo
	WebhookTargetURL string
}

// NewSalesService constructs a SalesService from its dependencies.
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

// CreateSale prices each line item from the current product/variant catalog,
// then persists a pending sale with its items in a transaction. It returns
// ErrEmptySaleItems when no items are supplied. No stock is reserved until the
// sale is confirmed.
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

// ConfirmSaleResult pairs the confirmed sale with the order created for it.
type ConfirmSaleResult struct {
	Sale  *models.Sale
	Order *models.Order
}

// SaleConfirmedCustomerPayload is the customer section of the sale.confirmed
// webhook payload.
type SaleConfirmedCustomerPayload struct {
	ID    *uuid.UUID `json:"id"`
	Name  string     `json:"name"`
	Email string     `json:"email"`
}

// SaleConfirmedWebhookPayload is the JSON body enqueued in the outbox and
// delivered to the configured webhook target when a sale is confirmed.
type SaleConfirmedWebhookPayload struct {
	Event       string                       `json:"event"`
	SaleID      uuid.UUID                    `json:"sale_id"`
	OrderID     uuid.UUID                    `json:"order_id"`
	OrderCode   string                       `json:"order_code"`
	BranchID    uuid.UUID                    `json:"branch_id"`
	Total       float64                      `json:"total"`
	Currency    string                       `json:"currency"`
	ConfirmedAt *time.Time                   `json:"confirmed_at"`
	Items       []models.SaleItem            `json:"items"`
	Customer    SaleConfirmedCustomerPayload `json:"customer"`
}

// ConfirmSale confirms a pending sale within a single transaction: it
// decrements stock for each item, marks the sale confirmed, creates an order
// with a generated code, and (when a webhook target is configured) enqueues a
// sale.confirmed event in the outbox. It returns ErrSaleNotPending if the sale
// is not in the pending state.
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
		customer := SaleConfirmedCustomerPayload{ID: confirmed.CustomerID}
		if confirmed.CustomerID != nil {
			contact, err := s.sales.FindCustomerContact(ctx, tx, *confirmed.CustomerID)
			if err != nil {
				return nil, err
			}
			customer.Name = contact.Name
			customer.Email = contact.Email
		}

		payload := buildSaleConfirmedWebhookPayload(confirmed, order, items, customer)
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

func buildSaleConfirmedWebhookPayload(
	confirmed *models.Sale,
	order *models.Order,
	items []models.SaleItem,
	customer SaleConfirmedCustomerPayload,
) SaleConfirmedWebhookPayload {
	return SaleConfirmedWebhookPayload{
		Event:       "sale.confirmed",
		SaleID:      confirmed.ID,
		OrderID:     order.ID,
		OrderCode:   order.Code,
		BranchID:    confirmed.BranchID,
		Total:       confirmed.Total,
		Currency:    confirmed.Currency,
		ConfirmedAt: confirmed.ConfirmedAt,
		Items:       items,
		Customer:    customer,
	}
}
