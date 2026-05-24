package middleware

import (
	"errors"

	"github.com/ficct-boutique/backend-go/internal/auth"
)

var ErrForbidden = errors.New("forbidden")
var ErrUnauthenticated = errors.New("unauthenticated")

func RequireRoles(claims *auth.Claims, roles ...auth.Role) error {
	if claims == nil {
		return ErrUnauthenticated
	}
	for _, r := range roles {
		if claims.Role == r {
			return nil
		}
	}
	return ErrForbidden
}
