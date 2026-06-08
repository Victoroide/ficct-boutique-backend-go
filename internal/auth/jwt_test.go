package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func loadTestKeys(t *testing.T) *KeyPair {
	t.Helper()
	kp, err := LoadKeyPair("../../.tools/keys/jwt_private_dev.pem", "../../.tools/keys/jwt_public_dev.pem", "dev-1")
	require.NoError(t, err, "keys should load (run from repo root)")
	return kp
}

func TestIssueAndVerify(t *testing.T) {
	kp := loadTestKeys(t)
	issuer, err := NewIssuer(kp, "ficct-go", []string{"ficct-angular"}, time.Minute)
	require.NoError(t, err)
	verifier := NewVerifier(kp, "ficct-go", "ficct-angular")

	uid := uuid.New()
	tok, exp, err := issuer.IssueAccess(uid, "victor@ficct.local", RoleAdmin, nil)
	require.NoError(t, err)
	require.True(t, exp.After(time.Now()))

	claims, err := verifier.Verify(tok)
	require.NoError(t, err)
	require.Equal(t, uid.String(), claims.Subject)
	require.Equal(t, RoleAdmin, claims.Role)
	require.Equal(t, "victor@ficct.local", claims.Email)
}

func TestLoadKeyPairFromPEMAcceptsEscapedNewlines(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicDER,
	})

	keys, err := LoadKeyPairFromPEM(
		strings.ReplaceAll(string(privatePEM), "\n", `\n`),
		strings.ReplaceAll(string(publicPEM), "\n", `\n`),
		"prod-1",
	)
	require.NoError(t, err)
	require.Equal(t, "prod-1", keys.KeyID)
	require.Equal(t, privateKey.N, keys.Private.N)
	require.Equal(t, privateKey.PublicKey.N, keys.Public.N)
}

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correctHorseBattery1")
	require.NoError(t, err)

	ok, err := VerifyPassword("correctHorseBattery1", hash)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = VerifyPassword("wrong", hash)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestPasswordMinLength(t *testing.T) {
	_, err := HashPassword("short")
	require.Error(t, err)
}
