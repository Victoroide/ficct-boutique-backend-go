package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignAndVerify(t *testing.T) {
	payload := []byte(`{"event":"sale.confirmed","sale_id":"abc"}`)
	sig := sign("super-secret", payload)
	require.NotEmpty(t, sig)
	require.True(t, Verify("super-secret", payload, sig))
	require.False(t, Verify("wrong-secret", payload, sig))
	require.False(t, Verify("super-secret", []byte("tampered"), sig))
}
