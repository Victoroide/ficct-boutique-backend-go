package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv        string
	AppPort       string
	LogLevel      string
	DatabaseURL   string
	MigrationsDir string

	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
	JWTPrivateKeyPEM  string
	JWTPublicKeyPEM   string
	JWTIssuer         string
	JWTAudience       []string
	JWTKeyID          string
	JWTAccessTTL      time.Duration
	JWTRefreshTTL     time.Duration

	CORSAllowedOrigins []string

	WebhookInvoiceURL       string
	WebhookHMACSecret       string
	WebhookDispatchInterval time.Duration
	WebhookMaxRetries       int

	RateLimitRPS   int
	RateLimitBurst int

	ExpoPushAPIURL      string
	ExpoPushAccessToken string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	accessTTL, err := strconv.Atoi(getEnvAny([]string{"JWT_ACCESS_TTL_MINUTES", "GO_JWT_ACCESS_TTL_MINUTES"}, "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TTL_MINUTES: %w", err)
	}
	refreshTTL, err := strconv.Atoi(getEnvAny([]string{"JWT_REFRESH_TTL_DAYS", "GO_JWT_REFRESH_TTL_DAYS"}, "7"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TTL_DAYS: %w", err)
	}
	dispatchInterval, err := strconv.Atoi(getEnvAny([]string{"WEBHOOK_DISPATCH_INTERVAL_SECONDS", "GO_WEBHOOK_DISPATCH_INTERVAL_SECONDS"}, "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WEBHOOK_DISPATCH_INTERVAL_SECONDS: %w", err)
	}
	maxRetries, err := strconv.Atoi(getEnvAny([]string{"WEBHOOK_MAX_RETRIES", "GO_WEBHOOK_MAX_RETRIES"}, "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WEBHOOK_MAX_RETRIES: %w", err)
	}
	rps, err := strconv.Atoi(getEnvAny([]string{"RATE_LIMIT_RPS", "GO_RATE_LIMIT_RPS"}, "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_RPS: %w", err)
	}
	burst, err := strconv.Atoi(getEnvAny([]string{"RATE_LIMIT_BURST", "GO_RATE_LIMIT_BURST"}, "40"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
	}

	dbURL := getEnvAny([]string{"DATABASE_URL", "GO_DATABASE_URL"}, "")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &Config{
		AppEnv:                  getEnvAny([]string{"APP_ENV", "GO_APP_ENV"}, "development"),
		AppPort:                 getEnvAny([]string{"APP_PORT", "PORT", "GO_APP_PORT"}, "8080"),
		LogLevel:                getEnvAny([]string{"APP_LOG_LEVEL", "GO_APP_LOG_LEVEL"}, "info"),
		DatabaseURL:             dbURL,
		MigrationsDir:           getEnvAny([]string{"MIGRATIONS_DIR", "GO_MIGRATIONS_DIR"}, "migrations"),
		JWTPrivateKeyPath:       getEnvAny([]string{"JWT_PRIVATE_KEY_PATH", "GO_JWT_PRIVATE_KEY_PATH"}, "/app/.tools/keys/jwt_private_dev.pem"),
		JWTPublicKeyPath:        getEnvAny([]string{"JWT_PUBLIC_KEY_PATH", "GO_JWT_PUBLIC_KEY_PATH"}, "/app/.tools/keys/jwt_public_dev.pem"),
		JWTPrivateKeyPEM:        getEnvAny([]string{"JWT_PRIVATE_KEY_PEM", "GO_JWT_PRIVATE_KEY_PEM"}, ""),
		JWTPublicKeyPEM:         getEnvAny([]string{"JWT_PUBLIC_KEY_PEM", "GO_JWT_PUBLIC_KEY_PEM"}, ""),
		JWTIssuer:               getEnvAny([]string{"JWT_ISSUER", "GO_JWT_ISSUER"}, "ficct-go"),
		JWTAudience:             splitCSV(getEnvAny([]string{"JWT_AUDIENCE", "GO_JWT_AUDIENCE"}, "ficct-express,ficct-django,ficct-angular,ficct-mobile")),
		JWTKeyID:                getEnvAny([]string{"JWT_KEY_ID", "GO_JWT_KEY_ID"}, "dev-1"),
		JWTAccessTTL:            time.Duration(accessTTL) * time.Minute,
		JWTRefreshTTL:           time.Duration(refreshTTL) * 24 * time.Hour,
		CORSAllowedOrigins:      splitCSV(getEnvAny([]string{"CORS_ALLOWED_ORIGINS", "GO_CORS_ALLOWED_ORIGINS"}, "http://localhost:4200")),
		WebhookInvoiceURL:       getEnvAny([]string{"WEBHOOK_INVOICE_URL", "GO_WEBHOOK_INVOICE_URL"}, ""),
		WebhookHMACSecret:       getEnvAny([]string{"WEBHOOK_HMAC_SECRET", "GO_WEBHOOK_HMAC_SECRET"}, ""),
		WebhookDispatchInterval: time.Duration(dispatchInterval) * time.Second,
		WebhookMaxRetries:       maxRetries,
		RateLimitRPS:            rps,
		RateLimitBurst:          burst,
		ExpoPushAPIURL:          getEnvAny([]string{"EXPO_PUSH_API_URL", "GO_EXPO_PUSH_API_URL"}, "https://exp.host/--/api/v2/push/send"),
		ExpoPushAccessToken:     getEnvAny([]string{"EXPO_PUSH_ACCESS_TOKEN", "GO_EXPO_PUSH_ACCESS_TOKEN"}, ""),
	}, nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvAny(keys []string, fallback string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
