package signing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeygenSignVerifyRoundTrip(t *testing.T) {
	priv, pub, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	payload := []byte("hello metaclaw")
	sig := Sign(payload, priv)
	if err := Verify(payload, sig, pub); err != nil {
		t.Fatalf("verify signature: %v", err)
	}

	if keyID := KeyIDFromPublicKey(pub); keyID == "" {
		t.Fatalf("expected key id")
	}
}

func TestPEMReadWrite(t *testing.T) {
	tmp := t.TempDir()
	priv, pub, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	privPath := filepath.Join(tmp, "k.priv.pem")
	pubPath := filepath.Join(tmp, "k.pub.pem")
	if err := WritePrivateKeyPEM(privPath, priv); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	if err := WritePublicKeyPEM(pubPath, pub); err != nil {
		t.Fatalf("write public key: %v", err)
	}
	if st, err := os.Stat(privPath); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected private key mode: err=%v mode=%v", err, st.Mode().Perm())
	}
	loadedPriv, err := LoadPrivateKeyPEM(privPath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	loadedPub, err := LoadPublicKeyPEM(pubPath)
	if err != nil {
		t.Fatalf("load public key: %v", err)
	}
	payload := []byte("payload")
	sig := Sign(payload, loadedPriv)
	if err := Verify(payload, sig, loadedPub); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
