package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ficct-boutique/backend-go/internal/models"
)

func TestCreateSaleRequiresItems(t *testing.T) {
	svc := NewSalesService(SalesServiceDeps{})
	_, _, err := svc.CreateSale(context.Background(), CreateSaleInput{
		BranchID: uuid.New(),
		Items:    nil,
	})
	require.ErrorIs(t, err, ErrEmptySaleItems)
}

func TestSaleConfirmedWebhookPayloadIncludesCustomerContact(t *testing.T) {
	confirmedAt := time.Date(2026, 6, 7, 18, 30, 0, 0, time.UTC)
	customerID := uuid.New()
	saleID := uuid.New()
	orderID := uuid.New()
	branchID := uuid.New()
	variantID := uuid.New()

	payload := buildSaleConfirmedWebhookPayload(
		&models.Sale{
			ID:          saleID,
			CustomerID:  &customerID,
			BranchID:    branchID,
			Total:       812.5,
			Currency:    "BOB",
			ConfirmedAt: &confirmedAt,
		},
		&models.Order{
			ID:   orderID,
			Code: "ORD-20260607-1234",
		},
		[]models.SaleItem{
			{
				ID:        uuid.New(),
				SaleID:    saleID,
				VariantID: variantID,
				Quantity:  2,
				UnitPrice: 350,
				LineTotal: 700,
			},
		},
		SaleConfirmedCustomerPayload{
			ID:    &customerID,
			Name:  "Maria Cliente",
			Email: "customer@example.test",
		},
	)

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "sale.confirmed", got["event"])
	require.Equal(t, saleID.String(), got["sale_id"])
	require.Equal(t, orderID.String(), got["order_id"])
	require.Equal(t, "ORD-20260607-1234", got["order_code"])
	require.Equal(t, branchID.String(), got["branch_id"])
	require.Equal(t, "BOB", got["currency"])
	require.Equal(t, 812.5, got["total"])
	require.NotEmpty(t, got["items"])

	customer, ok := got["customer"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, customerID.String(), customer["id"])
	require.Equal(t, "Maria Cliente", customer["name"])
	require.Equal(t, "customer@example.test", customer["email"])

	require.Contains(t, string(raw), `"VariantID"`, "item JSON keeps the existing SaleItem field casing")
}

func TestSaleConfirmedWebhookPayloadKeepsCustomerObjectForAnonymousSale(t *testing.T) {
	payload := buildSaleConfirmedWebhookPayload(
		&models.Sale{
			ID:       uuid.New(),
			BranchID: uuid.New(),
			Total:    100,
			Currency: "BOB",
		},
		&models.Order{
			ID:   uuid.New(),
			Code: "ORD-20260607-9999",
		},
		nil,
		SaleConfirmedCustomerPayload{},
	)

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	customer, ok := got["customer"].(map[string]interface{})
	require.True(t, ok)
	require.Nil(t, customer["id"])
	require.Equal(t, "", customer["name"])
	require.Equal(t, "", customer["email"])
}
