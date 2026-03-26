package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	cs := NewCryptoService()

	salt1, err := cs.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error = %v", err)
	}

	if len(salt1) != saltSize {
		t.Errorf("Salt length = %d, want %d", len(salt1), saltSize)
	}

	// Ensure salts are random (not the same)
	salt2, _ := cs.GenerateSalt()
	if bytes.Equal(salt1, salt2) {
		t.Error("GenerateSalt() returned identical salts, should be random")
	}
}

func TestGenerateNonce(t *testing.T) {
	cs := NewCryptoService()

	nonce1, err := cs.GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}

	if len(nonce1) != nonceSize {
		t.Errorf("Nonce length = %d, want %d", len(nonce1), nonceSize)
	}

	// Ensure nonces are random
	nonce2, _ := cs.GenerateNonce()
	if bytes.Equal(nonce1, nonce2) {
		t.Error("GenerateNonce() returned identical nonces, should be random")
	}
}

func TestDeriveKey(t *testing.T) {
	cs := NewCryptoService()
	salt, _ := cs.GenerateSalt()

	// Same password + salt should produce same key
	key1 := cs.DeriveKey("testpassword", salt)
	key2 := cs.DeriveKey("testpassword", salt)

	if !bytes.Equal(key1, key2) {
		t.Error("DeriveKey() with same inputs should produce same key")
	}

	// Key should be correct length for AES-256
	if len(key1) != argonKeyLen {
		t.Errorf("Key length = %d, want %d", len(key1), argonKeyLen)
	}

	// Different password should produce different key
	key3 := cs.DeriveKey("differentpassword", salt)
	if bytes.Equal(key1, key3) {
		t.Error("DeriveKey() with different passwords should produce different keys")
	}

	// Different salt should produce different key
	salt2, _ := cs.GenerateSalt()
	key4 := cs.DeriveKey("testpassword", salt2)
	if bytes.Equal(key1, key4) {
		t.Error("DeriveKey() with different salts should produce different keys")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	cs := NewCryptoService()

	plaintext := []byte("This is sensitive credential data: username=test, password=secret123")
	salt, _ := cs.GenerateSalt()
	nonce, _ := cs.GenerateNonce()
	key := cs.DeriveKey("masterpassword", salt)

	// Encrypt
	ciphertext, err := cs.Encrypt(plaintext, key, nonce)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Ciphertext should be longer (includes auth tag)
	if len(ciphertext) <= len(plaintext) {
		t.Error("Ciphertext should be longer than plaintext (includes auth tag)")
	}

	// Decrypt
	decrypted, err := cs.Decrypt(ciphertext, key, nonce)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Decrypted should match original
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted = %s, want %s", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	cs := NewCryptoService()

	plaintext := []byte("Sensitive data")
	salt, _ := cs.GenerateSalt()
	nonce, _ := cs.GenerateNonce()
	correctKey := cs.DeriveKey("correctpassword", salt)
	wrongKey := cs.DeriveKey("wrongpassword", salt)

	// Encrypt with correct key
	ciphertext, _ := cs.Encrypt(plaintext, correctKey, nonce)

	// Try to decrypt with wrong key - should fail
	_, err := cs.Decrypt(ciphertext, wrongKey, nonce)
	if err == nil {
		t.Error("Decrypt() with wrong key should fail")
	}
	if err != ErrDecryptionFailed {
		t.Errorf("Decrypt() error = %v, want ErrDecryptionFailed", err)
	}
}

func TestDecryptWithWrongNonce(t *testing.T) {
	cs := NewCryptoService()

	plaintext := []byte("Sensitive data")
	salt, _ := cs.GenerateSalt()
	nonce1, _ := cs.GenerateNonce()
	nonce2, _ := cs.GenerateNonce()
	key := cs.DeriveKey("password", salt)

	// Encrypt with nonce1
	ciphertext, _ := cs.Encrypt(plaintext, key, nonce1)

	// Try to decrypt with nonce2 - should fail
	_, err := cs.Decrypt(ciphertext, key, nonce2)
	if err == nil {
		t.Error("Decrypt() with wrong nonce should fail")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	cs := NewCryptoService()

	plaintext := []byte("Sensitive data")
	salt, _ := cs.GenerateSalt()
	nonce, _ := cs.GenerateNonce()
	key := cs.DeriveKey("password", salt)

	ciphertext, _ := cs.Encrypt(plaintext, key, nonce)

	// Tamper with ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[0] ^= 0xFF // Flip bits

	// Decryption should fail (GCM authentication)
	_, err := cs.Decrypt(tampered, key, nonce)
	if err == nil {
		t.Error("Decrypt() with tampered ciphertext should fail")
	}
}

func TestEncryptWithPassword(t *testing.T) {
	cs := NewCryptoService()

	plaintext := []byte("username:testuser\npassword:supersecret")
	password := "masterpassword123"

	salt, nonce, ciphertext, err := cs.EncryptWithPassword(plaintext, password)
	if err != nil {
		t.Fatalf("EncryptWithPassword() error = %v", err)
	}

	// All outputs should be non-empty
	if len(salt) == 0 || len(nonce) == 0 || len(ciphertext) == 0 {
		t.Error("EncryptWithPassword() returned empty values")
	}

	// Should be able to decrypt
	decrypted, err := cs.DecryptWithPassword(ciphertext, password, salt, nonce)
	if err != nil {
		t.Fatalf("DecryptWithPassword() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("DecryptWithPassword() did not return original plaintext")
	}
}

func TestDecryptWithPasswordWrongPassword(t *testing.T) {
	cs := NewCryptoService()

	plaintext := []byte("secret credentials")
	correctPassword := "correctpassword"
	wrongPassword := "wrongpassword"

	salt, nonce, ciphertext, _ := cs.EncryptWithPassword(plaintext, correctPassword)

	// Should fail with wrong password
	_, err := cs.DecryptWithPassword(ciphertext, wrongPassword, salt, nonce)
	if err == nil {
		t.Error("DecryptWithPassword() with wrong password should fail")
	}
}

func TestClearBytes(t *testing.T) {
	data := []byte("sensitive key material")
	original := make([]byte, len(data))
	copy(original, data)

	ClearBytes(data)

	// All bytes should be zero
	for i, b := range data {
		if b != 0 {
			t.Errorf("ClearBytes() did not clear byte at index %d", i)
		}
	}

	// Should not equal original
	if bytes.Equal(data, original) {
		t.Error("ClearBytes() did not modify the slice")
	}
}

func TestBase64EncodeDecode(t *testing.T) {
	original := []byte("test data with special chars: !@#$%")

	encoded := EncodeBase64(original)
	decoded, err := DecodeBase64(encoded)
	if err != nil {
		t.Fatalf("DecodeBase64() error = %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Error("Base64 encode/decode roundtrip failed")
	}
}

// Benchmark tests
func BenchmarkDeriveKey(b *testing.B) {
	cs := NewCryptoService()
	salt, _ := cs.GenerateSalt()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.DeriveKey("benchmarkpassword", salt)
	}
}

func BenchmarkEncrypt(b *testing.B) {
	cs := NewCryptoService()
	plaintext := []byte("benchmark test data for encryption performance")
	salt, _ := cs.GenerateSalt()
	nonce, _ := cs.GenerateNonce()
	key := cs.DeriveKey("password", salt)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.Encrypt(plaintext, key, nonce)
	}
}
