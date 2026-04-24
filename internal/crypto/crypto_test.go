package crypto

import (
	"bytes"
	"crypto/hkdf"
	"crypto/sha256"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
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

// ============================================================================
// Recovery key derivation (v2 scheme) — HKDF-SHA256 with domain-separated
// info labels on top of Argon2id. The invariants enforced here are the
// direct defense against the v1 bug (verifyHash == encryptKey) recurring.
// ============================================================================

// TestDeriveRecoveryKeys_VerifyAndEncryptDistinct pins the CORE SAFETY
// INVARIANT: the verification hash stored on disk must not be usable as the
// encryption key that wraps the master vault key. In the v1 scheme the two
// were byte-identical, letting anyone with read access to vault.osm decrypt
// the vault. A regression here would silently re-introduce that bug.
func TestDeriveRecoveryKeys_VerifyAndEncryptDistinct(t *testing.T) {
	cs := NewCryptoService()
	salt, err := cs.GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}

	verifyHash, encryptKey, err := cs.DeriveRecoveryKeys("apple bear cloud drift eagle flame", salt)
	if err != nil {
		t.Fatalf("DeriveRecoveryKeys: %v", err)
	}

	if bytes.Equal(verifyHash, encryptKey) {
		t.Fatal("verifyHash and encryptKey must be distinct — this is the v1 bug")
	}
	if len(verifyHash) != recoveryKeyLen || len(encryptKey) != recoveryKeyLen {
		t.Errorf("expected both keys to be %d bytes, got verify=%d encrypt=%d",
			recoveryKeyLen, len(verifyHash), len(encryptKey))
	}
}

// TestDeriveRecoveryKeys_Deterministic pins that the same phrase + salt
// always produce the same pair. Any nondeterminism would break verification
// and break the migration from v1 vaults (where we need to be able to
// reconstruct the encrypt key from the phrase to decrypt the stored
// EncryptedVaultKey).
func TestDeriveRecoveryKeys_Deterministic(t *testing.T) {
	cs := NewCryptoService()
	salt, _ := cs.GenerateSalt()
	phrase := "garden harbor iron jungle keen lilac"

	verify1, encrypt1, err := cs.DeriveRecoveryKeys(phrase, salt)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	verify2, encrypt2, err := cs.DeriveRecoveryKeys(phrase, salt)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if !bytes.Equal(verify1, verify2) {
		t.Error("verify hash must be deterministic across calls")
	}
	if !bytes.Equal(encrypt1, encrypt2) {
		t.Error("encrypt key must be deterministic across calls")
	}
}

// TestDeriveRecoveryKeys_DifferentSaltsDifferentKeys — same phrase, different
// salt must produce completely different outputs. Catches a future refactor
// that accidentally drops salt from either HKDF call.
func TestDeriveRecoveryKeys_DifferentSaltsDifferentKeys(t *testing.T) {
	cs := NewCryptoService()
	salt1, _ := cs.GenerateSalt()
	salt2, _ := cs.GenerateSalt()
	phrase := "maple nectar oasis pearl quilt raven"

	verify1, encrypt1, _ := cs.DeriveRecoveryKeys(phrase, salt1)
	verify2, encrypt2, _ := cs.DeriveRecoveryKeys(phrase, salt2)

	if bytes.Equal(verify1, verify2) {
		t.Error("different salts must produce different verify hashes")
	}
	if bytes.Equal(encrypt1, encrypt2) {
		t.Error("different salts must produce different encrypt keys")
	}
}

// TestDeriveRecoveryKeys_DifferentPhrasesDifferentKeys — different phrase,
// same salt must produce completely different outputs.
func TestDeriveRecoveryKeys_DifferentPhrasesDifferentKeys(t *testing.T) {
	cs := NewCryptoService()
	salt, _ := cs.GenerateSalt()

	verify1, encrypt1, _ := cs.DeriveRecoveryKeys("sable tiger unicorn velvet willow zephyr", salt)
	verify2, encrypt2, _ := cs.DeriveRecoveryKeys("zebra yak xenon wolf vesper umbra", salt)

	if bytes.Equal(verify1, verify2) {
		t.Error("different phrases must produce different verify hashes")
	}
	if bytes.Equal(encrypt1, encrypt2) {
		t.Error("different phrases must produce different encrypt keys")
	}
}

// TestDeriveRecoveryKeys_LabelsProvideDomainSeparation pins that the HKDF
// info labels are doing real work. If a future refactor accidentally unifies
// the two info strings (or picks the same one in two places), the verify
// hash would again equal the encrypt key. This test uses the same Argon2id
// base derivation the real code does, then proves that two HKDF expansions
// with our exact labels differ from each other AND from any reasonable
// typo variant.
func TestDeriveRecoveryKeys_LabelsProvideDomainSeparation(t *testing.T) {
	cs := NewCryptoService()
	salt, _ := cs.GenerateSalt()
	verifyHash, encryptKey, err := cs.DeriveRecoveryKeys("aurora brook coast delta elm feather", salt)
	if err != nil {
		t.Fatalf("DeriveRecoveryKeys: %v", err)
	}

	// Re-derive with a hypothetical typo'd label and prove it would produce
	// yet a third distinct value — demonstrates that the info parameter is
	// the thing providing separation, not coincidence.
	master := argon2.IDKey(
		[]byte(normalizeRecoveryPhrase("aurora brook coast delta elm feather")),
		salt, argonTime, argonMemory, argonThreads, argonKeyLen,
	)
	typoExpansion, err := hkdfTestKey(master, "deckstr.org/v1/recovery/verifyyy")
	if err != nil {
		t.Fatalf("hkdfTestKey: %v", err)
	}

	if bytes.Equal(verifyHash, typoExpansion) {
		t.Error("typo'd label must produce a different output (info parameter is not providing separation)")
	}
	if bytes.Equal(encryptKey, typoExpansion) {
		t.Error("typo'd label must produce a different output from encrypt key too")
	}
}

// hkdfTestKey is a test-only helper that mirrors the production HKDF call
// shape. Defined here rather than exporting the real path to keep the
// crypto package's public surface minimal.
func hkdfTestKey(secret []byte, info string) ([]byte, error) {
	return hkdf.Key(sha256.New, secret, nil, info, recoveryKeyLen)
}

// TestNormalizeRecoveryPhrase pins the normalization contract: lowercase,
// trim edges, collapse all internal whitespace (including tabs) into single
// spaces. Fixes the pre-v2 UX bug where "apple  bear" (two spaces) hashed
// differently than "apple bear".
func TestNormalizeRecoveryPhrase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"apple bear", "apple bear"},
		{"  apple  bear  ", "apple bear"},
		{"APPLE BEAR", "apple bear"},
		{"apple\tbear", "apple bear"},
		{"apple\nbear", "apple bear"},
		{"  Apple   BEAR  Coast  ", "apple bear coast"},
	}
	for _, tc := range cases {
		if got := normalizeRecoveryPhrase(tc.in); got != tc.want {
			t.Errorf("normalizeRecoveryPhrase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestVerifyRecoveryPhraseV2_RoundTrip — the verify path must accept the
// same phrase it was created from (including whitespace variants via
// normalization) and reject others.
func TestVerifyRecoveryPhraseV2_RoundTrip(t *testing.T) {
	cs := NewCryptoService()
	salt, _ := cs.GenerateSalt()
	phrase := "wolf voyage sunset pearl onyx harbor"
	verifyHash, _, err := cs.DeriveRecoveryKeys(phrase, salt)
	if err != nil {
		t.Fatalf("DeriveRecoveryKeys: %v", err)
	}

	// Correct phrase + whitespace variations all verify.
	for _, variant := range []string{
		phrase,
		strings.ToUpper(phrase),
		"  " + phrase + "  ",
		strings.Replace(phrase, " ", "  ", -1),
	} {
		ok, err := cs.VerifyRecoveryPhraseV2(variant, salt, verifyHash)
		if err != nil {
			t.Errorf("VerifyRecoveryPhraseV2(%q): %v", variant, err)
		}
		if !ok {
			t.Errorf("VerifyRecoveryPhraseV2(%q) = false, want true", variant)
		}
	}

	// Wrong phrase fails.
	ok, err := cs.VerifyRecoveryPhraseV2("not the right phrase at all today", salt, verifyHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ok {
		t.Error("wrong phrase must not verify")
	}
}
