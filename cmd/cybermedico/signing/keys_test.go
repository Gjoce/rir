package signing_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"cybermedico/cmd/cybermedico/signing"
)

func newCipher(t *testing.T) *signing.KeyCipher {
	t.Helper()
	key := make([]byte, signing.SigningKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, err := signing.NewKeyCipher(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("NewKeyCipher: %v", err)
	}

	return c
}

func TestKeyPairSignsAndVerifies(t *testing.T) {
	c := newCipher(t)
	pub, encrypted, err := c.GenerateKeyPair("doctor-1")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key is %d bytes", len(pub))
	}

	priv, err := c.PrivateKey("doctor-1", pub, encrypted)
	if err != nil {
		t.Fatalf("PrivateKey: %v", err)
	}
	defer signing.ZeroPrivateKey(priv)

	msg := []byte("opinion")
	if !ed25519.Verify(pub, msg, ed25519.Sign(priv, msg)) {
		t.Fatal("signature from the decrypted key doesn't verify with the public key")
	}
}

func TestPrivateKeyIsNotStoredInPlain(t *testing.T) {
	c := newCipher(t)
	pub, encrypted, err := c.GenerateKeyPair("doctor-1")
	if err != nil {
		t.Fatal(err)
	}
	priv, err := c.PrivateKey("doctor-1", pub, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	seed := priv.Seed()
	for i := 0; i+len(seed) <= len(encrypted); i++ {
		if string(encrypted[i:i+len(seed)]) == string(seed) {
			t.Fatal("the encrypted blob contains the plain private key")
		}
	}
}

func TestPrivateKeyRefusals(t *testing.T) {
	c := newCipher(t)
	pub, encrypted, err := c.GenerateKeyPair("doctor-1")
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := c.GenerateKeyPair("doctor-2")
	if err != nil {
		t.Fatal(err)
	}

	tampered := append([]byte(nil), encrypted...)
	tampered[len(tampered)-1] ^= 1

	if _, err := c.PrivateKey("doctor-2", pub, encrypted); !errors.Is(err, signing.ErrPrivateKeyDecrypt) {
		t.Errorf("another doctor's row: err = %v", err)
	}
	if _, err := c.PrivateKey("doctor-1", pub, tampered); !errors.Is(err, signing.ErrPrivateKeyDecrypt) {
		t.Errorf("tampered blob: err = %v", err)
	}
	if _, err := newCipher(t).PrivateKey("doctor-1", pub, encrypted); !errors.Is(err, signing.ErrPrivateKeyDecrypt) {
		t.Errorf("wrong signing key: err = %v", err)
	}
	if _, err := c.PrivateKey("doctor-1", pub, encrypted[:5]); !errors.Is(err, signing.ErrPrivateKeyDecrypt) {
		t.Errorf("short blob: err = %v", err)
	}
	if _, err := c.PrivateKey("doctor-1", otherPub, encrypted); err == nil {
		t.Error("a swapped public key was accepted")
	}
}

func TestNewKeyCipherRejectsBadKeys(t *testing.T) {
	for _, key := range []string{"", "not base64!", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if _, err := signing.NewKeyCipher(key); !errors.Is(err, signing.ErrInvalidSigningKey) {
			t.Errorf("key %q: err = %v", key, err)
		}
	}
}
