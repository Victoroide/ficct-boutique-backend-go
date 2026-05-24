package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ficct-boutique/backend-go/internal/auth"
)

type ctxKey string

const claimsCtxKey ctxKey = "ficct.claims"

func RequireAuth(verifier *auth.TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := extractBearer(r)
			if tok == "" {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			claims, err := verifier.Verify(tok)
			if err != nil {
				http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuth(verifier *auth.TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := extractBearer(r)
			if tok != "" {
				if claims, err := verifier.Verify(tok); err == nil {
					ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsCtxKey).(*auth.Claims)
	return c, ok
}

// ContextWithClaims is a test helper that injects claims into a context using
// the same key as the HTTP middleware. Production code should never call
// this; it exists so unit tests can exercise resolver RBAC without minting a
// real JWT or running an HTTP server.
func ContextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsCtxKey, claims)
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
