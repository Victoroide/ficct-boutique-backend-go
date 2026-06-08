package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadAcceptsGoPrefixedDeploymentEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GO_DATABASE_URL", "postgres://ficct:ficct@example:5432/ficct")
	t.Setenv("APP_PORT", "")
	t.Setenv("PORT", "")
	t.Setenv("GO_APP_PORT", "9090")
	t.Setenv("JWT_PRIVATE_KEY_PEM", "")
	t.Setenv("GO_JWT_PRIVATE_KEY_PEM", "private-pem")
	t.Setenv("JWT_PUBLIC_KEY_PEM", "")
	t.Setenv("GO_JWT_PUBLIC_KEY_PEM", "public-pem")
	t.Setenv("JWT_KEY_ID", "")
	t.Setenv("GO_JWT_KEY_ID", "prod-1")
	t.Setenv("MIGRATIONS_DIR", "migrations")
	t.Setenv("WEBHOOK_INVOICE_URL", "")
	t.Setenv("GO_WEBHOOK_INVOICE_URL", "https://n8n.example/webhook/ficct-invoice")
	t.Setenv("WEBHOOK_HMAC_SECRET", "")
	t.Setenv("GO_WEBHOOK_HMAC_SECRET", "secret")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "postgres://ficct:ficct@example:5432/ficct", cfg.DatabaseURL)
	require.Equal(t, "9090", cfg.AppPort)
	require.Equal(t, "private-pem", cfg.JWTPrivateKeyPEM)
	require.Equal(t, "public-pem", cfg.JWTPublicKeyPEM)
	require.Equal(t, "prod-1", cfg.JWTKeyID)
	require.Equal(t, "migrations", cfg.MigrationsDir)
	require.Equal(t, "https://n8n.example/webhook/ficct-invoice", cfg.WebhookInvoiceURL)
	require.Equal(t, "secret", cfg.WebhookHMACSecret)
}
