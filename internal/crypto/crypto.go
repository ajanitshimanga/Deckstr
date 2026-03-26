package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters (OWASP recommended)
	argonTime    = 3         // Number of iterations
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4         // Parallelism
	argonKeyLen  = 32        // 256 bits for AES-256

	// Salt and nonce sizes
	saltSize  = 16 // 128 bits
	nonceSize = 12 // 96 bits for GCM
)

var (
	ErrDecryptionFailed = errors.New("decryption failed: invalid password or corrupted data")
	ErrInvalidData      = errors.New("invalid encrypted data format")
)

// CryptoService handles encryption and key derivation
type CryptoService struct{}

// NewCryptoService creates a new crypto service instance
func NewCryptoService() *CryptoService {
	return &CryptoService{}
}

// GenerateSalt creates a random salt for key derivation
func (c *CryptoService) GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateNonce creates a random nonce for AES-GCM
func (c *CryptoService) GenerateNonce() ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return nonce, nil
}

// DeriveKey derives an encryption key from a password using Argon2id
func (c *CryptoService) DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)
}

// Encrypt encrypts plaintext using AES-256-GCM
func (c *CryptoService) Encrypt(plaintext []byte, key []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(nonce), gcm.NonceSize())
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (c *CryptoService) Decrypt(ciphertext []byte, key []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(nonce), gcm.NonceSize())
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptWithPassword is a convenience method that handles key derivation
func (c *CryptoService) EncryptWithPassword(plaintext []byte, password string) (salt, nonce, ciphertext []byte, err error) {
	salt, err = c.GenerateSalt()
	if err != nil {
		return nil, nil, nil, err
	}

	nonce, err = c.GenerateNonce()
	if err != nil {
		return nil, nil, nil, err
	}

	key := c.DeriveKey(password, salt)
	ciphertext, err = c.Encrypt(plaintext, key, nonce)
	if err != nil {
		return nil, nil, nil, err
	}

	return salt, nonce, ciphertext, nil
}

// DecryptWithPassword is a convenience method that handles key derivation
func (c *CryptoService) DecryptWithPassword(ciphertext []byte, password string, salt, nonce []byte) ([]byte, error) {
	key := c.DeriveKey(password, salt)
	return c.Decrypt(ciphertext, key, nonce)
}

// EncodeBase64 encodes bytes to base64 string
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes base64 string to bytes
func DecodeBase64(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

// ClearBytes zeros out a byte slice (for clearing sensitive data from memory)
func ClearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
