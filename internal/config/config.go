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
	AppEnv      string
	AppPort     string
	LogLevel    string
	DatabaseURL string

	JWTPrivateKeyPath string
	JWTPublicKeyPath  string
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

	accessTTL, err := strconv.Atoi(getEnv("JWT_ACCESS_TTL_MINUTES", "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TTL_MINUTES: %w", err)
	}
	refreshTTL, err := strconv.Atoi(getEnv("JWT_REFRESH_TTL_DAYS", "7"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TTL_DAYS: %w", err)
	}
	dispatchInterval, err := strconv.Atoi(getEnv("WEBHOOK_DISPATCH_INTERVAL_SECONDS", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WEBHOOK_DISPATCH_INTERVAL_SECONDS: %w", err)
	}
	maxRetries, err := strconv.Atoi(getEnv("WEBHOOK_MAX_RETRIES", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WEBHOOK_MAX_RETRIES: %w", err)
	}
	rps, err := strconv.Atoi(getEnv("RATE_LIMIT_RPS", "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_RPS: %w", err)
	}
	burst, err := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "40"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &Config{
		AppEnv:                  getEnv("APP_ENV", "development"),
		AppPort:                 getEnv("APP_PORT", "8080"),
		LogLevel:                getEnv("APP_LOG_LEVEL", "info"),
		DatabaseURL:             dbURL,
		JWTPrivateKeyPath:       getEnv("JWT_PRIVATE_KEY_PATH", "/app/.tools/keys/jwt_private_dev.pem"),
		JWTPublicKeyPath:        getEnv("JWT_PUBLIC_KEY_PATH", "/app/.tools/keys/jwt_public_dev.pem"),
		JWTIssuer:               getEnv("JWT_ISSUER", "ficct-go"),
		JWTAudience:             splitCSV(getEnv("JWT_AUDIENCE", "ficct-express,ficct-django,ficct-angular,ficct-mobile")),
		JWTKeyID:                getEnv("JWT_KEY_ID", "dev-1"),
		JWTAccessTTL:            time.Duration(accessTTL) * time.Minute,
		JWTRefreshTTL:           time.Duration(refreshTTL) * 24 * time.Hour,
		CORSAllowedOrigins:      splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:4200")),
		WebhookInvoiceURL:       os.Getenv("WEBHOOK_INVOICE_URL"),
		WebhookHMACSecret:       os.Getenv("WEBHOOK_HMAC_SECRET"),
		WebhookDispatchInterval: time.Duration(dispatchInterval) * time.Second,
		WebhookMaxRetries:       maxRetries,
		RateLimitRPS:            rps,
		RateLimitBurst:          burst,
		ExpoPushAPIURL:          getEnv("EXPO_PUSH_API_URL", "https://exp.host/--/api/v2/push/send"),
		ExpoPushAccessToken:     os.Getenv("EXPO_PUSH_ACCESS_TOKEN"),
	}, nil
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
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
