package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	privateKeyPEMType = "PRIVATE KEY"
	publicKeyPEMType  = "PUBLIC KEY"
)

func GenerateEd25519KeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, pub, nil
}

func WritePrivateKeyPEM(path string, key ed25519.PrivateKey) error {
	if len(key) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid ed25519 private key size: %d", len(key))
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	block := &pem.Block{Type: privateKeyPEMType, Bytes: pkcs8}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
}

func WritePublicKeyPEM(path string, key ed25519.PublicKey) error {
	if len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid ed25519 public key size: %d", len(key))
	}
	spki, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}
	block := &pem.Block{Type: publicKeyPEMType, Bytes: spki}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0o644)
}

func LoadPrivateKeyPEM(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM: %s", path)
	}
	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	priv, ok := pk.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ed25519")
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key size: %d", len(priv))
	}
	return priv, nil
}

func LoadPublicKeyPEM(path string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM: %s", path)
	}
	pk, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := pk.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not ed25519")
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key size: %d", len(pub))
	}
	return pub, nil
}

func KeyIDFromPublicKey(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "ed25519:" + hex.EncodeToString(sum[:8])
}

func Sign(payload []byte, privateKey ed25519.PrivateKey) string {
	sig := ed25519.Sign(privateKey, payload)
	return base64.StdEncoding.EncodeToString(sig)
}

func Verify(payload []byte, signatureBase64 string, publicKey ed25519.PublicKey) error {
	sig, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(publicKey, payload, sig) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}
