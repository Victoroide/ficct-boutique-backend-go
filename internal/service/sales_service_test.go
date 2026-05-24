package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateSaleRequiresItems(t *testing.T) {
	svc := NewSalesService(SalesServiceDeps{})
	_, _, err := svc.CreateSale(context.Background(), CreateSaleInput{
		BranchID: uuid.New(),
		Items:    nil,
	})
	require.ErrorIs(t, err, ErrEmptySaleItems)
}
