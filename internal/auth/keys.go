package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

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
	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return key, nil
}

func loadPublic(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key %q: %w", path, err)
	}
	key, err := jwt.ParseRSAPublicKeyFromPEM(data)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return key, nil
}
