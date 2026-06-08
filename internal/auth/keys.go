package auth

import (
	"crypto/rsa"
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type KeyPair struct {
	Private *rsa.PrivateKey
	Public  *rsa.PublicKey
	KeyID   string
}

func LoadKeyPair(privatePath, publicPath, keyID string) (*KeyPair, error) {
	priv, err := loadPrivate(privatePath)
	if err != nil {
		return nil, err
	}
	pub, err := loadPublic(publicPath)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Private: priv, Public: pub, KeyID: keyID}, nil
}

func LoadKeyPairFromPEM(privatePEM, publicPEM, keyID string) (*KeyPair, error) {
	privatePEM = normalizePEM(privatePEM)
	publicPEM = normalizePEM(publicPEM)
	if privatePEM == "" {
		return nil, fmt.Errorf("JWT private key PEM is required")
	}
	priv, err := parsePrivate([]byte(privatePEM))
	if err != nil {
		return nil, err
	}
	pub := &priv.PublicKey
	if publicPEM != "" {
		pub, err = parsePublic([]byte(publicPEM))
		if err != nil {
			return nil, err
		}
	}
	return &KeyPair{Private: priv, Public: pub, KeyID: keyID}, nil
}

func LoadPublicOnly(publicPath, keyID string) (*KeyPair, error) {
	pub, err := loadPublic(publicPath)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Public: pub, KeyID: keyID}, nil
}

func loadPrivate(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key %q: %w", path, err)
	}
	return parsePrivate(data)
}

func loadPublic(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key %q: %w", path, err)
	}
	return parsePublic(data)
}

func parsePrivate(data []byte) (*rsa.PrivateKey, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return key, nil
}

func parsePublic(data []byte) (*rsa.PublicKey, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return key, nil
}

func normalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, `\n`, "\n")
}
