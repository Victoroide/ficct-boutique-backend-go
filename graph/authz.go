package graph

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ficct-boutique/backend-go/internal/auth"
	"github.com/ficct-boutique/backend-go/internal/middleware"
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

// isAdminOrStaff reports whether the authenticated caller has the admin or
// staff role, without producing an authorization error.
func isAdminOrStaff(ctx context.Context) bool {
	claims, ok := middleware.ClaimsFromContext(ctx)
	return ok && claims != nil && (claims.Role == auth.RoleAdmin || claims.Role == auth.RoleStaff)
}

// subjectUUID returns the authenticated caller's user id from the JWT subject.
func subjectUUID(ctx context.Context) (uuid.UUID, error) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return uuid.Nil, ErrUnauthenticated
	}
	return uuid.Parse(claims.Subject)
}
