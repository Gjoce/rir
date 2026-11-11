package signing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// SigningKeySize is the length of the app key that encrypts doctors' private keys.
const SigningKeySize = 32

var (
	// ErrInvalidSigningKey means SIGNING_KEY isn't 32 bytes of base64.
	ErrInvalidSigningKey = errors.New("signing: the signing key must be 32 bytes, base64 encoded")
	// ErrPrivateKeyDecrypt means a stored private key can't be decrypted: the wrong
	// app key, a damaged blob, or a blob that belongs to another doctor.
	ErrPrivateKeyDecrypt = errors.New("signing: can't decrypt the private key")
)

// KeyCipher encrypts doctors' Ed25519 private keys with the app's signing key
// (AES-256-GCM), so the database alone isn't enough to sign in a doctor's name.
// Each blob is bound to its doctor: a private key copied to another doctor's
// row doesn't decrypt.
type KeyCipher struct {
	aead cipher.AEAD
}

// NewKeyCipher builds a KeyCipher from the base64 encoded 32-byte signing key.
func NewKeyCipher(encodedKey string) (*KeyCipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil || len(key) != SigningKeySize {
		return nil, ErrInvalidSigningKey
	}
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("signing: create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("signing: create GCM: %w", err)
	}

	return &KeyCipher{aead: aead}, nil
}

// GenerateKeyPair creates a doctor's Ed25519 key pair. The public key is stored
// as is; the private key only encrypted.
func (c *KeyCipher) GenerateKeyPair(doctorID string) (publicKey, encryptedPrivateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("signing: generate key pair: %w", err)
	}
	defer zeroBytes(priv)

	encrypted, err := c.encrypt(doctorID, priv.Seed())
	if err != nil {
		return nil, nil, err
	}

	return pub, encrypted, nil
}

// PrivateKey decrypts a doctor's private key and checks it belongs to
// publicKey. Callers zero it with ZeroPrivateKey once they're done signing.
func (c *KeyCipher) PrivateKey(doctorID string, publicKey, encryptedPrivateKey []byte) (ed25519.PrivateKey, error) {
	seed, err := c.decrypt(doctorID, encryptedPrivateKey)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(seed)

	if len(seed) != ed25519.SeedSize {
		return nil, ErrPrivateKeyDecrypt
	}
	priv := ed25519.NewKeyFromSeed(seed)

	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok || !pub.Equal(ed25519.PublicKey(publicKey)) {
		zeroBytes(priv)

		return nil, errors.New("signing: the private key doesn't match the public key")
	}

	return priv, nil
}

// ZeroPrivateKey removes key material from memory.
func ZeroPrivateKey(priv ed25519.PrivateKey) {
	zeroBytes(priv)
}

// encrypt returns nonce || ciphertext, with the doctor's ID as additional data.
func (c *KeyCipher) encrypt(doctorID string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("signing: generate nonce: %w", err)
	}

	return c.aead.Seal(nonce, nonce, plaintext, []byte(doctorID)), nil
}

func (c *KeyCipher) decrypt(doctorID string, blob []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(blob) < nonceSize {
		return nil, ErrPrivateKeyDecrypt
	}

	plaintext, err := c.aead.Open(nil, blob[:nonceSize], blob[nonceSize:], []byte(doctorID))
	if err != nil {
		return nil, ErrPrivateKeyDecrypt
	}

	return plaintext, nil
}

// zeroBytes overwrites b with zeros to remove key material from memory.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
