package graph

import (
	"context"
	"errors"

	"github.com/ficct-boutique/backend-go/internal/auth"
	"github.com/ficct-boutique/backend-go/internal/middleware"
	"github.com/google/uuid"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
)

func requireAuth(ctx context.Context) error {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return ErrUnauthenticated
	}
	return nil
}

func requireAdmin(ctx context.Context) error {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return ErrUnauthenticated
	}
	if claims.Role != auth.RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func requireAdminOrStaff(ctx context.Context) error {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return ErrUnauthenticated
	}
	if claims.Role != auth.RoleAdmin && claims.Role != auth.RoleStaff {
		return ErrForbidden
	}
	return nil
}

func parseUUIDFromClaim(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
