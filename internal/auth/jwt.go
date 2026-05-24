package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleStaff    Role = "staff"
	RoleCustomer Role = "customer"
	RoleSystem   Role = "system"
)

type Claims struct {
	Role     Role       `json:"role"`
	BranchID *uuid.UUID `json:"branch_id,omitempty"`
	Email    string     `json:"email"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	keys      *KeyPair
	issuer    string
	audience  []string
	accessTTL time.Duration
}

func NewIssuer(keys *KeyPair, issuer string, audience []string, accessTTL time.Duration) (*TokenIssuer, error) {
	if keys.Private == nil {
		return nil, errors.New("issuer requires a private key")
	}
	return &TokenIssuer{
		keys:      keys,
		issuer:    issuer,
		audience:  audience,
		accessTTL: accessTTL,
	}, nil
}

func (t *TokenIssuer) IssueAccess(userID uuid.UUID, email string, role Role, branchID *uuid.UUID) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(t.accessTTL)
	claims := Claims{
		Role:     role,
		BranchID: branchID,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings(t.audience),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = t.keys.KeyID
	signed, err := tok.SignedString(t.keys.Private)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

type TokenVerifier struct {
	keys     *KeyPair
	issuer   string
	audience string
}

func NewVerifier(keys *KeyPair, issuer, audience string) *TokenVerifier {
	return &TokenVerifier{keys: keys, issuer: issuer, audience: audience}
}

func (v *TokenVerifier) Verify(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.keys.Public, nil
	}, jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
