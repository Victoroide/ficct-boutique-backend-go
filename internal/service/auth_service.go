package service

import (
	"context"
	"errors"
	"time"

	"github.com/ficct-boutique/backend-go/internal/auth"
	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInactiveUser       = errors.New("user is inactive")
	ErrEmailExists        = errors.New("email already exists")
)

type AuthService struct {
	users  *repository.UserRepo
	issuer *auth.TokenIssuer
}

func NewAuthService(users *repository.UserRepo, issuer *auth.TokenIssuer) *AuthService {
	return &AuthService{users: users, issuer: issuer}
}

type AuthResult struct {
	User        *models.User
	AccessToken string
	ExpiresAt   time.Time
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}
	ok, err := auth.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	tok, exp, err := s.issuer.IssueAccess(user.ID, user.Email, auth.Role(user.Role), nil)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, AccessToken: tok, ExpiresAt: exp}, nil
}

func (s *AuthService) Register(ctx context.Context, email, password, fullName string, role models.Role) (*models.User, error) {
	existing, err := s.users.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailExists
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	return s.users.Create(ctx, email, hash, fullName, role)
}
