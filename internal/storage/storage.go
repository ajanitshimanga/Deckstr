package storage

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"OpenSmurfManager/internal/appdir"
	"OpenSmurfManager/internal/crypto"
	"OpenSmurfManager/internal/models"
)

const (
	// vaultFileName is intentionally unchanged across the Deckstr rebrand —
	// the .osm extension lives inside the per-user data directory and isn't
	// user-facing, so renaming it would force a second migration with no UX
	// benefit.
	vaultFileName = "vault.osm"

	// vaultVersion is the semantics version new vaults are stamped with. It
	// changes whenever the cryptographic scheme changes in a way that older
	// clients cannot safely handle. Migration is handled in migrateVault.
	//
	//   v1 — initial release; recovery-phrase verification hash was the same
	//        bytes as the master-vault-key wrapping key (critical bug, see
	//        docs/security/recovery-key-derivation.md).
	//   v2 — recovery scheme derives verify and encrypt keys via HKDF-SHA256
	//        with domain-separated info labels on top of a single Argon2id
	//        base step. v1 vaults are migrated on next unlock.
	vaultVersion = 2
)

var (
	ErrVaultNotFound       = errors.New("vault not found")
	ErrVaultLocked         = errors.New("vault is locked")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrInvalidUsername     = errors.New("invalid username")
	ErrVaultExists         = errors.New("vault already exists")
	// ErrUnsupportedVersion is returned when the on-disk vault was written by
	// a newer build than this one. Forward-compat fence: refusing the load is
	// safer than letting an old binary silently corrupt newer-format data.
	ErrUnsupportedVersion = errors.New("vault version is newer than this build supports — update Deckstr")
)

// StorageService handles encrypted vault operations
type StorageService struct {
	crypto      *crypto.CryptoService
	vaultPath   string
	mu          sync.RWMutex

	// Unlocked state
	isUnlocked   bool
	username     string // Current logged-in username
	passwordHint string // Password hint (stored unencrypted)
	derivedKey   []byte
	salt         []byte
	vaultData    *models.VaultData

	// createdAt is the original vault creation timestamp, preserved across all
	// mutations. Populated on Unlock / CreateVault*; any Save/Change path that
	// overwrote this with time.Now() used to silently reset account history.
	createdAt time.Time

	// Recovery phrase fields (preserved across saves)
	recoveryPhraseHash string
	recoveryPhraseSalt string
	recoveryKeyNonce   string
	encryptedVaultKey  string

	// needsRecoveryRotation is set by migrateVault when a v1 vault is loaded.
	// Unlock consumes the flag after successful master-password decryption
	// and rotates the recovery scheme to v2 in-place.
	needsRecoveryRotation bool

	// pendingRotatedPhrase holds the cleartext of a newly-generated recovery
	// phrase that the frontend has not yet displayed to the user. Set by the
	// v1→v2 migration in Unlock and consumed exactly once by the frontend
	// through ConsumePendingRecoveryRotation, which clears it from memory
	// immediately on read.
	pendingRotatedPhrase string
}

// NewStorageService creates a new storage service
func NewStorageService() (*StorageService, error) {
	vaultPath, err := getVaultPath()
	if err != nil {
		return nil, err
	}

	return NewStorageServiceWithPath(vaultPath), nil
}

// NewStorageServiceWithPath creates a storage service backed by the given
// vault file path. Intended for tests and tooling that need to control the
// storage location instead of using the OS config directory.
func NewStorageServiceWithPath(vaultPath string) *StorageService {
	return &StorageService{
		crypto:    crypto.NewCryptoService(),
		vaultPath: vaultPath,
	}
}

// getVaultPath returns the path to the vault file. The directory resolution
// (and any rebrand migration) is delegated to internal/appdir so storage and
// telemetry agree on a single canonical location.
func getVaultPath() (string, error) {
	dir, err := appdir.Path()
	if err != nil {
		return "", fmt.Errorf("failed to resolve app directory: %w", err)
	}
	return filepath.Join(dir, vaultFileName), nil
}

// VaultExists checks if a vault file exists
func (s *StorageService) VaultExists() bool {
	_, err := os.Stat(s.vaultPath)
	return err == nil
}

// IsUnlocked returns whether the vault is currently unlocked
func (s *StorageService) IsUnlocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isUnlocked
}

// CreateVault creates a new vault with the given username and master password
func (s *StorageService) CreateVault(username, masterPassword string) error {
	return s.CreateVaultWithHint(username, masterPassword, "")
}

// CreateVaultWithHint creates a new vault with an optional password hint
func (s *StorageService) CreateVaultWithHint(username, masterPassword, hint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.VaultExists() {
		return ErrVaultExists
	}

	// Initialize with default data
	vaultData := models.NewVaultData()

	// Serialize vault data
	plaintext, err := json.Marshal(vaultData)
	if err != nil {
		return fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt
	salt, nonce, ciphertext, err := s.crypto.EncryptWithPassword(plaintext, masterPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Create vault structure
	now := time.Now()
	vault := models.Vault{
		Version:       vaultVersion,
		Username:      username,
		PasswordHint:  hint,
		Salt:          crypto.EncodeBase64(salt),
		Nonce:         crypto.EncodeBase64(nonce),
		EncryptedData: crypto.EncodeBase64(ciphertext),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Save to file
	if err := s.saveVaultFile(vault); err != nil {
		return err
	}

	// Keep vault unlocked after creation
	s.isUnlocked = true
	s.username = username
	s.passwordHint = hint
	s.derivedKey = s.crypto.DeriveKey(masterPassword, salt)
	s.salt = salt
	s.vaultData = &vaultData
	s.createdAt = now

	return nil
}

// Unlock decrypts and loads the vault with the given username and master password
func (s *StorageService) Unlock(username, masterPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.VaultExists() {
		return ErrVaultNotFound
	}

	// Load vault file
	vault, err := s.loadVaultFile()
	if err != nil {
		return err
	}

	// Verify username matches
	if vault.Username != username {
		return ErrInvalidUsername
	}

	// Decode base64 values
	salt, err := crypto.DecodeBase64(vault.Salt)
	if err != nil {
		return fmt.Errorf("invalid salt: %w", err)
	}

	nonce, err := crypto.DecodeBase64(vault.Nonce)
	if err != nil {
		return fmt.Errorf("invalid nonce: %w", err)
	}

	ciphertext, err := crypto.DecodeBase64(vault.EncryptedData)
	if err != nil {
		return fmt.Errorf("invalid encrypted data: %w", err)
	}

	// Decrypt
	plaintext, err := s.crypto.DecryptWithPassword(ciphertext, masterPassword, salt, nonce)
	if err != nil {
		if errors.Is(err, crypto.ErrDecryptionFailed) {
			return ErrInvalidPassword
		}
		return err
	}

	// Deserialize
	var vaultData models.VaultData
	if err := json.Unmarshal(plaintext, &vaultData); err != nil {
		return fmt.Errorf("failed to deserialize vault data: %w", err)
	}

	// Always regenerate GameNetworks from defaults (not user-customizable, avoids schema migration issues)
	vaultData.GameNetworks = models.DefaultGameNetworks()

	// Store unlocked state
	s.isUnlocked = true
	s.username = username
	s.passwordHint = vault.PasswordHint
	s.derivedKey = s.crypto.DeriveKey(masterPassword, salt)
	s.salt = salt
	s.vaultData = &vaultData
	s.createdAt = vault.CreatedAt

	// Preserve recovery phrase fields for Save() operations
	s.recoveryPhraseHash = vault.RecoveryPhraseHash
	s.recoveryPhraseSalt = vault.RecoveryPhraseSalt
	s.recoveryKeyNonce = vault.RecoveryKeyNonce
	s.encryptedVaultKey = vault.EncryptedVaultKey

	// If the vault was v1, rotate the recovery scheme to v2 before returning.
	// This path only runs with a correct master password, so we won't rewrite
	// on a failed unlock. The rotation regenerates the recovery phrase —
	// the old phrase dies here. Frontend surfaces the new phrase via
	// ConsumePendingRecoveryRotation.
	if s.needsRecoveryRotation {
		if err := s.rotateRecoveryToV2Locked(); err != nil {
			// Rotation failed — leave the vault on v1 rather than partial
			// state. User retries on next unlock. We do NOT return this as
			// an unlock error because the vault IS unlocked; rotation is a
			// best-effort side effect.
			s.needsRecoveryRotation = false
			return fmt.Errorf("vault unlocked but recovery rotation failed (retry on next unlock): %w", err)
		}
	}

	return nil
}

// rotateRecoveryToV2Locked rewrites the vault from the v1 recovery scheme
// to v2 using a freshly-generated phrase. Caller must hold s.mu for writing
// and s.derivedKey / s.salt / s.vaultData must already be populated by the
// just-completed Unlock path.
//
// Side effects on success:
//   - vault.osm is atomically rewritten with Version=2 and new recovery fields
//   - s.recoveryPhrase* / s.encryptedVaultKey are updated to the new values
//   - s.pendingRotatedPhrase is set for the frontend to consume
//   - s.needsRecoveryRotation is cleared
func (s *StorageService) rotateRecoveryToV2Locked() error {
	newPhrase, err := s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		return fmt.Errorf("generate phrase: %w", err)
	}
	newRecoverySalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("generate recovery salt: %w", err)
	}

	verifyHash, encryptKey, err := s.crypto.DeriveRecoveryKeys(newPhrase, newRecoverySalt)
	if err != nil {
		return fmt.Errorf("derive recovery keys: %w", err)
	}
	defer crypto.ClearBytes(encryptKey)

	newRecoveryKeyNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return fmt.Errorf("generate recovery nonce: %w", err)
	}

	// Wrap the existing master vault key with the new v2 encrypt key. The
	// master key itself doesn't change — only the recovery wrapper.
	encryptedVaultKey, err := s.crypto.Encrypt(s.derivedKey, encryptKey, newRecoveryKeyNonce)
	if err != nil {
		return fmt.Errorf("encrypt vault key: %w", err)
	}

	// Re-serialize the vault data under the existing master key. We have to
	// regenerate the nonce because GCM nonces must never repeat under a
	// given key — even though the plaintext is identical to what's on disk.
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return fmt.Errorf("serialize vault data: %w", err)
	}
	newNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return fmt.Errorf("generate data nonce: %w", err)
	}
	newCiphertext, err := s.crypto.Encrypt(plaintext, s.derivedKey, newNonce)
	if err != nil {
		return fmt.Errorf("encrypt vault data: %w", err)
	}

	vault := models.Vault{
		Version:            vaultVersion, // stamps v2
		Username:           s.username,
		PasswordHint:       s.passwordHint,
		Salt:               crypto.EncodeBase64(s.salt),
		Nonce:              crypto.EncodeBase64(newNonce),
		EncryptedData:      crypto.EncodeBase64(newCiphertext),
		RecoveryPhraseHash: crypto.EncodeBase64(verifyHash),
		RecoveryPhraseSalt: crypto.EncodeBase64(newRecoverySalt),
		RecoveryKeyNonce:   crypto.EncodeBase64(newRecoveryKeyNonce),
		EncryptedVaultKey:  crypto.EncodeBase64(encryptedVaultKey),
		CreatedAt:          s.createdAt,
		UpdatedAt:          time.Now(),
	}
	if err := s.saveVaultFile(vault); err != nil {
		return fmt.Errorf("save migrated vault: %w", err)
	}

	// Update in-memory state to match on-disk.
	s.recoveryPhraseHash = vault.RecoveryPhraseHash
	s.recoveryPhraseSalt = vault.RecoveryPhraseSalt
	s.recoveryKeyNonce = vault.RecoveryKeyNonce
	s.encryptedVaultKey = vault.EncryptedVaultKey
	s.pendingRotatedPhrase = newPhrase
	s.needsRecoveryRotation = false
	return nil
}

// ConsumePendingRecoveryRotation returns the cleartext of a recovery phrase
// that was just generated during a v1→v2 migration, if one is pending. The
// phrase is returned at most ONCE per unlock — subsequent calls return
// ("", false) so the frontend can't accidentally display it twice and the
// cleartext doesn't linger in Go memory beyond the first read.
func (s *StorageService) ConsumePendingRecoveryRotation() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingRotatedPhrase == "" {
		return "", false
	}
	phrase := s.pendingRotatedPhrase
	s.pendingRotatedPhrase = ""
	return phrase, true
}

// Lock clears the unlocked vault from memory
func (s *StorageService) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.derivedKey != nil {
		crypto.ClearBytes(s.derivedKey)
	}
	s.isUnlocked = false
	s.username = ""
	s.passwordHint = ""
	s.derivedKey = nil
	s.salt = nil
	s.vaultData = nil
	s.createdAt = time.Time{}
	s.pendingRotatedPhrase = ""
	s.needsRecoveryRotation = false
}

// GetUsername returns the current logged-in username
func (s *StorageService) GetUsername() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.username
}

// GetStoredUsername returns the username stored in the vault file (without unlocking)
func (s *StorageService) GetStoredUsername() (string, error) {
	if !s.VaultExists() {
		return "", ErrVaultNotFound
	}
	vault, err := s.loadVaultFile()
	if err != nil {
		return "", err
	}
	return vault.Username, nil
}

// GetPasswordHint returns the password hint stored in the vault file (without unlocking)
func (s *StorageService) GetPasswordHint() (string, error) {
	if !s.VaultExists() {
		return "", ErrVaultNotFound
	}
	vault, err := s.loadVaultFile()
	if err != nil {
		return "", err
	}
	return vault.PasswordHint, nil
}

// UpdatePasswordHint updates the password hint without changing the password
// This allows legacy users to add a hint to their existing vault
func (s *StorageService) UpdatePasswordHint(hint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return ErrVaultLocked
	}

	// Update in-memory hint
	s.passwordHint = hint

	// Generate new nonce for the save
	nonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return err
	}

	// Serialize vault data
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt with existing derived key
	ciphertext, err := s.crypto.Encrypt(plaintext, s.derivedKey, nonce)
	if err != nil {
		return fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Update vault structure with new hint (preserving recovery phrase fields)
	vault := models.Vault{
		Version:            vaultVersion,
		Username:           s.username,
		PasswordHint:       hint,
		Salt:               crypto.EncodeBase64(s.salt),
		Nonce:              crypto.EncodeBase64(nonce),
		EncryptedData:      crypto.EncodeBase64(ciphertext),
		RecoveryPhraseHash: s.recoveryPhraseHash,
		RecoveryPhraseSalt: s.recoveryPhraseSalt,
		RecoveryKeyNonce:   s.recoveryKeyNonce,
		EncryptedVaultKey:  s.encryptedVaultKey,
		CreatedAt:          s.createdAt,
		UpdatedAt:          time.Now(),
	}

	return s.saveVaultFile(vault)
}

// Save encrypts and persists the current vault data
func (s *StorageService) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return ErrVaultLocked
	}

	// Generate new nonce for each save
	nonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return err
	}

	// Serialize vault data
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt with existing derived key
	ciphertext, err := s.crypto.Encrypt(plaintext, s.derivedKey, nonce)
	if err != nil {
		return fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Update vault structure (preserving recovery phrase fields + original creation time)
	vault := models.Vault{
		Version:            vaultVersion,
		Username:           s.username,
		PasswordHint:       s.passwordHint,
		Salt:               crypto.EncodeBase64(s.salt),
		Nonce:              crypto.EncodeBase64(nonce),
		EncryptedData:      crypto.EncodeBase64(ciphertext),
		RecoveryPhraseHash: s.recoveryPhraseHash,
		RecoveryPhraseSalt: s.recoveryPhraseSalt,
		RecoveryKeyNonce:   s.recoveryKeyNonce,
		EncryptedVaultKey:  s.encryptedVaultKey,
		CreatedAt:          s.createdAt,
		UpdatedAt:          time.Now(),
	}

	return s.saveVaultFile(vault)
}

// ChangePassword re-encrypts the vault with a new password
// The user must be unlocked and provide the correct current password
// Returns a new recovery phrase (old one is invalidated)
func (s *StorageService) ChangePassword(currentPassword, newPassword string) (newRecoveryPhrase string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return "", ErrVaultLocked
	}

	// Verify current password by re-deriving the key and comparing against
	// the unlocked-state copy. Constant-time compare prevents a timing oracle
	// on byte position; our existing VerifyRecoveryPhrase already uses
	// subtle — this path was inconsistent with that.
	testKey := s.crypto.DeriveKey(currentPassword, s.salt)
	defer crypto.ClearBytes(testKey)

	if subtle.ConstantTimeCompare(testKey, s.derivedKey) != 1 {
		return "", ErrInvalidPassword
	}

	// Generate new salt and nonce for the new password
	newSalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate new salt: %w", err)
	}

	newNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate new nonce: %w", err)
	}

	// Derive new key from new password
	newKey := s.crypto.DeriveKey(newPassword, newSalt)

	// Serialize vault data
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return "", fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt with new key
	ciphertext, err := s.crypto.Encrypt(plaintext, newKey, newNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Generate NEW recovery phrase (old one is invalidated)
	newRecoveryPhrase, err = s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery phrase: %w", err)
	}

	// Generate salt for the new recovery phrase
	newRecoverySalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery salt: %w", err)
	}

	// Derive the verify hash + distinct encrypt key from the phrase via HKDF
	// (v2 scheme). verifyHash is stored on disk; encryptKey wraps newKey and
	// is never persisted directly.
	newRecoveryHash, newRecoveryKey, err := s.crypto.DeriveRecoveryKeys(newRecoveryPhrase, newRecoverySalt)
	if err != nil {
		return "", fmt.Errorf("failed to derive recovery keys: %w", err)
	}
	defer crypto.ClearBytes(newRecoveryKey)

	// Generate nonce for encrypting the vault key
	newRecoveryKeyNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery key nonce: %w", err)
	}

	// Encrypt the new vault key with the new recovery-derived encrypt key
	newEncryptedVaultKey, err := s.crypto.Encrypt(newKey, newRecoveryKey, newRecoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault key: %w", err)
	}

	// Create new vault structure
	vault := models.Vault{
		Version:            vaultVersion,
		Username:           s.username,
		PasswordHint:       s.passwordHint,
		Salt:               crypto.EncodeBase64(newSalt),
		Nonce:              crypto.EncodeBase64(newNonce),
		EncryptedData:      crypto.EncodeBase64(ciphertext),
		RecoveryPhraseHash: crypto.EncodeBase64(newRecoveryHash),
		RecoveryPhraseSalt: crypto.EncodeBase64(newRecoverySalt),
		RecoveryKeyNonce:   crypto.EncodeBase64(newRecoveryKeyNonce),
		EncryptedVaultKey:  crypto.EncodeBase64(newEncryptedVaultKey),
		CreatedAt:          s.createdAt,
		UpdatedAt:          time.Now(),
	}

	// Save to file
	if err := s.saveVaultFile(vault); err != nil {
		return "", err
	}

	// Clear old key and update state
	crypto.ClearBytes(s.derivedKey)
	s.derivedKey = newKey
	s.salt = newSalt

	return newRecoveryPhrase, nil
}

// GetVaultData returns a copy of the current vault data
func (s *StorageService) GetVaultData() (*models.VaultData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isUnlocked {
		return nil, ErrVaultLocked
	}

	// Return a copy to prevent external modifications
	dataCopy := *s.vaultData
	return &dataCopy, nil
}

// UpdateVaultData updates the vault data (caller should call Save after)
func (s *StorageService) UpdateVaultData(data *models.VaultData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return ErrVaultLocked
	}

	s.vaultData = data
	return nil
}

// loadVaultFile reads the vault from disk
func (s *StorageService) loadVaultFile() (*models.Vault, error) {
	data, err := os.ReadFile(s.vaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrVaultNotFound
		}
		return nil, fmt.Errorf("failed to read vault file: %w", err)
	}

	var vault models.Vault
	if err := json.Unmarshal(data, &vault); err != nil {
		return nil, fmt.Errorf("failed to parse vault file: %w", err)
	}

	// Run any necessary migrations
	if err := s.migrateVault(&vault); err != nil {
		return nil, fmt.Errorf("failed to migrate vault: %w", err)
	}

	return &vault, nil
}

// migrateVault inspects the on-disk vault's schema version and either flags
// it for migration or refuses the load. Idempotent: running twice on the same
// vault produces the same result.
//
//   v1 — flagged for recovery-scheme rotation on next successful Unlock
//   v2 — current, no-op
//   >2 — forward-compat fence. Refusing is safer than a silent downgrade
//        where an old binary writes v2-scheme fields over v3-format data.
func (s *StorageService) migrateVault(vault *models.Vault) error {
	switch {
	case vault.Version == vaultVersion:
		return nil
	case vault.Version == 1:
		// v1 → v2 recovery-scheme rotation. The rewrite happens in Unlock
		// after the master password has decrypted the vault, so we only
		// touch disk when we have the key material needed to re-wrap it.
		s.needsRecoveryRotation = true
		return nil
	case vault.Version > vaultVersion:
		return fmt.Errorf("%w (on-disk version %d, this build handles up to %d)",
			ErrUnsupportedVersion, vault.Version, vaultVersion)
	default:
		// Should be unreachable given the two equality cases above, but
		// treating an unexpected low number as unsupported is the safer
		// default than silently opening it.
		return fmt.Errorf("%w (on-disk version %d)", ErrUnsupportedVersion, vault.Version)
	}
}

// saveVaultFile writes the vault to disk
func (s *StorageService) saveVaultFile(vault models.Vault) error {
	data, err := json.MarshalIndent(vault, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize vault: %w", err)
	}

	// Write atomically using temp file
	tmpPath := s.vaultPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write vault file: %w", err)
	}

	if err := os.Rename(tmpPath, s.vaultPath); err != nil {
		os.Remove(tmpPath) // Clean up on error
		return fmt.Errorf("failed to save vault file: %w", err)
	}

	return nil
}

// GetVaultPath returns the path to the vault file (for debugging/info)
func (s *StorageService) GetVaultPath() string {
	return s.vaultPath
}

// HasRecoveryPhrase checks if the vault has a recovery phrase set (without unlocking)
func (s *StorageService) HasRecoveryPhrase() (bool, error) {
	if !s.VaultExists() {
		return false, ErrVaultNotFound
	}
	vault, err := s.loadVaultFile()
	if err != nil {
		return false, err
	}
	return vault.RecoveryPhraseHash != "" && vault.RecoveryPhraseSalt != "", nil
}

// CreateVaultWithRecoveryPhrase creates a new vault and returns the recovery phrase
// The phrase should be shown to the user once (hidden by default) and stored securely
func (s *StorageService) CreateVaultWithRecoveryPhrase(username, masterPassword, hint string) (recoveryPhrase string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.VaultExists() {
		return "", ErrVaultExists
	}

	// Generate recovery phrase
	recoveryPhrase, err = s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery phrase: %w", err)
	}

	// Generate salt for recovery phrase hash
	recoverySalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery salt: %w", err)
	}

	// Derive verify hash + distinct encrypt key via HKDF (v2 scheme).
	recoveryHash, recoveryKey, err := s.crypto.DeriveRecoveryKeys(recoveryPhrase, recoverySalt)
	if err != nil {
		return "", fmt.Errorf("failed to derive recovery keys: %w", err)
	}
	defer crypto.ClearBytes(recoveryKey)

	// Initialize with default data
	vaultData := models.NewVaultData()

	// Serialize vault data
	plaintext, err := json.Marshal(vaultData)
	if err != nil {
		return "", fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt vault data with password-derived key
	salt, nonce, ciphertext, err := s.crypto.EncryptWithPassword(plaintext, masterPassword)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Derive the vault key (we need to store this encrypted with recovery key)
	vaultKey := s.crypto.DeriveKey(masterPassword, salt)

	// Generate nonce for encrypting the vault key
	recoveryKeyNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery key nonce: %w", err)
	}

	// Encrypt the vault key with the recovery-derived key
	encryptedVaultKey, err := s.crypto.Encrypt(vaultKey, recoveryKey, recoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault key: %w", err)
	}

	// Create vault structure with recovery phrase
	now := time.Now()
	vault := models.Vault{
		Version:            vaultVersion,
		Username:           username,
		PasswordHint:       hint,
		Salt:               crypto.EncodeBase64(salt),
		Nonce:              crypto.EncodeBase64(nonce),
		EncryptedData:      crypto.EncodeBase64(ciphertext),
		RecoveryPhraseHash: crypto.EncodeBase64(recoveryHash),
		RecoveryPhraseSalt: crypto.EncodeBase64(recoverySalt),
		RecoveryKeyNonce:   crypto.EncodeBase64(recoveryKeyNonce),
		EncryptedVaultKey:  crypto.EncodeBase64(encryptedVaultKey),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Save to file
	if err := s.saveVaultFile(vault); err != nil {
		return "", err
	}

	// Keep vault unlocked after creation
	s.isUnlocked = true
	s.username = username
	s.passwordHint = hint
	s.derivedKey = vaultKey
	s.salt = salt
	s.vaultData = &vaultData
	s.createdAt = now

	return recoveryPhrase, nil
}

// GenerateRecoveryPhraseForLegacyUser generates a recovery phrase for an existing user without one
// Must be called while the vault is unlocked. Returns the new recovery phrase.
func (s *StorageService) GenerateRecoveryPhraseForLegacyUser() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return "", ErrVaultLocked
	}

	// Generate recovery phrase
	recoveryPhrase, err := s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery phrase: %w", err)
	}

	// Generate salt for recovery phrase hash
	recoverySalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery salt: %w", err)
	}

	// Derive verify hash + distinct encrypt key via HKDF (v2 scheme).
	recoveryHash, recoveryKey, err := s.crypto.DeriveRecoveryKeys(recoveryPhrase, recoverySalt)
	if err != nil {
		return "", fmt.Errorf("failed to derive recovery keys: %w", err)
	}
	defer crypto.ClearBytes(recoveryKey)

	// Generate nonce for encrypting the vault key
	recoveryKeyNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery key nonce: %w", err)
	}

	// Encrypt the current vault key with the recovery-derived encrypt key
	encryptedVaultKey, err := s.crypto.Encrypt(s.derivedKey, recoveryKey, recoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault key: %w", err)
	}

	// Generate new nonce for the vault data save
	nonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", err
	}

	// Serialize vault data
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return "", fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt with existing derived key
	ciphertext, err := s.crypto.Encrypt(plaintext, s.derivedKey, nonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Update vault structure with recovery phrase
	vault := models.Vault{
		Version:            vaultVersion,
		Username:           s.username,
		PasswordHint:       s.passwordHint,
		Salt:               crypto.EncodeBase64(s.salt),
		Nonce:              crypto.EncodeBase64(nonce),
		EncryptedData:      crypto.EncodeBase64(ciphertext),
		RecoveryPhraseHash: crypto.EncodeBase64(recoveryHash),
		RecoveryPhraseSalt: crypto.EncodeBase64(recoverySalt),
		RecoveryKeyNonce:   crypto.EncodeBase64(recoveryKeyNonce),
		EncryptedVaultKey:  crypto.EncodeBase64(encryptedVaultKey),
		CreatedAt:          s.createdAt,
		UpdatedAt:          time.Now(),
	}

	if err := s.saveVaultFile(vault); err != nil {
		return "", err
	}

	// Update in-memory fields so subsequent Save() calls preserve them
	s.recoveryPhraseHash = vault.RecoveryPhraseHash
	s.recoveryPhraseSalt = vault.RecoveryPhraseSalt
	s.recoveryKeyNonce = vault.RecoveryKeyNonce
	s.encryptedVaultKey = vault.EncryptedVaultKey

	return recoveryPhrase, nil
}

// RegenerateRecoveryPhrase verifies the password and generates a new recovery phrase
// This requires password verification as a security measure since the recovery phrase
// is a master key backup that can reset the vault password
func (s *StorageService) RegenerateRecoveryPhrase(password string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return "", ErrVaultLocked
	}

	// Verify the password by attempting to decrypt
	vault, err := s.loadVaultFile()
	if err != nil {
		return "", err
	}

	salt, err := crypto.DecodeBase64(vault.Salt)
	if err != nil {
		return "", fmt.Errorf("invalid salt: %w", err)
	}

	nonce, err := crypto.DecodeBase64(vault.Nonce)
	if err != nil {
		return "", fmt.Errorf("invalid nonce: %w", err)
	}

	ciphertext, err := crypto.DecodeBase64(vault.EncryptedData)
	if err != nil {
		return "", fmt.Errorf("invalid encrypted data: %w", err)
	}

	// Verify password by attempting decryption
	_, err = s.crypto.DecryptWithPassword(ciphertext, password, salt, nonce)
	if err != nil {
		if errors.Is(err, crypto.ErrDecryptionFailed) {
			return "", ErrInvalidPassword
		}
		return "", err
	}

	// Password verified, now generate new recovery phrase
	recoveryPhrase, err := s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery phrase: %w", err)
	}

	// Generate salt for recovery phrase hash
	recoverySalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery salt: %w", err)
	}

	// Derive verify hash + distinct encrypt key via HKDF (v2 scheme).
	recoveryHash, recoveryKey, err := s.crypto.DeriveRecoveryKeys(recoveryPhrase, recoverySalt)
	if err != nil {
		return "", fmt.Errorf("failed to derive recovery keys: %w", err)
	}
	defer crypto.ClearBytes(recoveryKey)

	// Generate nonce for encrypting the vault key
	recoveryKeyNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate recovery key nonce: %w", err)
	}

	// Encrypt the current vault key with the recovery-derived encrypt key
	encryptedVaultKey, err := s.crypto.Encrypt(s.derivedKey, recoveryKey, recoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault key: %w", err)
	}

	// Generate new nonce for the vault data save
	newNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", err
	}

	// Serialize vault data
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return "", fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt with existing derived key
	newCiphertext, err := s.crypto.Encrypt(plaintext, s.derivedKey, newNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Update vault structure with new recovery phrase
	updatedVault := models.Vault{
		Version:            vaultVersion,
		Username:           s.username,
		PasswordHint:       s.passwordHint,
		Salt:               crypto.EncodeBase64(s.salt),
		Nonce:              crypto.EncodeBase64(newNonce),
		EncryptedData:      crypto.EncodeBase64(newCiphertext),
		RecoveryPhraseHash: crypto.EncodeBase64(recoveryHash),
		RecoveryPhraseSalt: crypto.EncodeBase64(recoverySalt),
		RecoveryKeyNonce:   crypto.EncodeBase64(recoveryKeyNonce),
		EncryptedVaultKey:  crypto.EncodeBase64(encryptedVaultKey),
		CreatedAt:          vault.CreatedAt,
		UpdatedAt:          time.Now(),
	}

	if err := s.saveVaultFile(updatedVault); err != nil {
		return "", err
	}

	// Update in-memory fields so subsequent Save() calls preserve them
	s.recoveryPhraseHash = updatedVault.RecoveryPhraseHash
	s.recoveryPhraseSalt = updatedVault.RecoveryPhraseSalt
	s.recoveryKeyNonce = updatedVault.RecoveryKeyNonce
	s.encryptedVaultKey = updatedVault.EncryptedVaultKey

	return recoveryPhrase, nil
}

// ResetPasswordWithRecoveryPhrase resets the password using the recovery phrase
// This validates the phrase, re-encrypts with a new password, and generates a new recovery phrase
// Returns the new recovery phrase
func (s *StorageService) ResetPasswordWithRecoveryPhrase(recoveryPhrase, newPassword, newHint string) (newRecoveryPhrase string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.VaultExists() {
		return "", ErrVaultNotFound
	}

	// Load vault file
	vault, err := s.loadVaultFile()
	if err != nil {
		return "", err
	}

	// Check if recovery phrase is set
	if vault.RecoveryPhraseHash == "" || vault.RecoveryPhraseSalt == "" || vault.EncryptedVaultKey == "" {
		return "", errors.New("no recovery phrase set for this vault")
	}

	// Decode recovery phrase salt
	recoverySalt, err := crypto.DecodeBase64(vault.RecoveryPhraseSalt)
	if err != nil {
		return "", fmt.Errorf("invalid recovery salt: %w", err)
	}

	// Decode stored hash for verification
	storedHash, err := crypto.DecodeBase64(vault.RecoveryPhraseHash)
	if err != nil {
		return "", fmt.Errorf("invalid recovery hash: %w", err)
	}

	// Verify phrase and derive the encrypt key for the vault key, branching
	// on the vault schema version. In the v1 scheme both operations use the
	// same single Argon2id output (that's the bug this PR fixes). In v2 they
	// are distinct HKDF expansions.
	var recoveryKey []byte
	switch vault.Version {
	case 1:
		if !s.crypto.VerifyRecoveryPhrase(recoveryPhrase, recoverySalt, storedHash) {
			return "", errors.New("invalid recovery phrase")
		}
		// v1: the stored hash IS the encrypt key (the bug). Use the legacy
		// derivation so we can unwrap the v1 EncryptedVaultKey one last time
		// before re-encrypting everything under v2 below.
		recoveryKey = s.crypto.HashRecoveryPhrase(recoveryPhrase, recoverySalt)
	case 2:
		var verifyHash []byte
		verifyHash, recoveryKey, err = s.crypto.DeriveRecoveryKeys(recoveryPhrase, recoverySalt)
		if err != nil {
			return "", fmt.Errorf("failed to derive recovery keys: %w", err)
		}
		defer crypto.ClearBytes(verifyHash)
		if subtle.ConstantTimeCompare(verifyHash, storedHash) != 1 {
			return "", errors.New("invalid recovery phrase")
		}
	default:
		return "", ErrUnsupportedVersion
	}
	defer crypto.ClearBytes(recoveryKey)

	// Decode encrypted vault key and its nonce
	encryptedVaultKey, err := crypto.DecodeBase64(vault.EncryptedVaultKey)
	if err != nil {
		return "", fmt.Errorf("invalid encrypted vault key: %w", err)
	}

	recoveryKeyNonce, err := crypto.DecodeBase64(vault.RecoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("invalid recovery key nonce: %w", err)
	}

	// Decrypt the vault key using the recovery-derived key
	vaultKey, err := s.crypto.Decrypt(encryptedVaultKey, recoveryKey, recoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt vault key: %w", err)
	}

	// Now decrypt the vault data using the recovered vault key
	nonce, err := crypto.DecodeBase64(vault.Nonce)
	if err != nil {
		return "", fmt.Errorf("invalid nonce: %w", err)
	}

	ciphertext, err := crypto.DecodeBase64(vault.EncryptedData)
	if err != nil {
		return "", fmt.Errorf("invalid encrypted data: %w", err)
	}

	plaintext, err := s.crypto.Decrypt(ciphertext, vaultKey, nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt vault data: %w", err)
	}

	// Deserialize vault data
	var vaultData models.VaultData
	if err := json.Unmarshal(plaintext, &vaultData); err != nil {
		return "", fmt.Errorf("failed to deserialize vault data: %w", err)
	}

	// Clear the old vault key
	crypto.ClearBytes(vaultKey)

	// Now re-encrypt everything with the new password
	// Generate new salt for the new password
	newSalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate new salt: %w", err)
	}

	// Derive new key from new password
	newVaultKey := s.crypto.DeriveKey(newPassword, newSalt)

	// Generate new nonce for vault data
	newNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate new nonce: %w", err)
	}

	// Re-encrypt vault data with new key
	newCiphertext, err := s.crypto.Encrypt(plaintext, newVaultKey, newNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Generate NEW recovery phrase (old one is invalidated)
	newRecoveryPhrase, err = s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		return "", fmt.Errorf("failed to generate new recovery phrase: %w", err)
	}

	// Generate new salt for the new recovery phrase
	newRecoverySalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return "", fmt.Errorf("failed to generate new recovery salt: %w", err)
	}

	// Derive verify hash + distinct encrypt key for the NEW phrase via v2
	// HKDF. Regardless of whether the incoming vault was v1 or v2, the
	// vault is written back as v2 below.
	newRecoveryHash, newRecoveryKey, err := s.crypto.DeriveRecoveryKeys(newRecoveryPhrase, newRecoverySalt)
	if err != nil {
		return "", fmt.Errorf("failed to derive new recovery keys: %w", err)
	}
	defer crypto.ClearBytes(newRecoveryKey)

	// Generate nonce for encrypting the new vault key
	newRecoveryKeyNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate new recovery key nonce: %w", err)
	}

	// Encrypt the new vault key with the new recovery-derived encrypt key
	newEncryptedVaultKey, err := s.crypto.Encrypt(newVaultKey, newRecoveryKey, newRecoveryKeyNonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt new vault key: %w", err)
	}

	// Create new vault structure
	newVault := models.Vault{
		Version:            vaultVersion,
		Username:           vault.Username,
		PasswordHint:       newHint,
		Salt:               crypto.EncodeBase64(newSalt),
		Nonce:              crypto.EncodeBase64(newNonce),
		EncryptedData:      crypto.EncodeBase64(newCiphertext),
		RecoveryPhraseHash: crypto.EncodeBase64(newRecoveryHash),
		RecoveryPhraseSalt: crypto.EncodeBase64(newRecoverySalt),
		RecoveryKeyNonce:   crypto.EncodeBase64(newRecoveryKeyNonce),
		EncryptedVaultKey:  crypto.EncodeBase64(newEncryptedVaultKey),
		CreatedAt:          vault.CreatedAt,
		UpdatedAt:          time.Now(),
	}

	// Save the new vault
	if err := s.saveVaultFile(newVault); err != nil {
		return "", err
	}

	// Update in-memory state (vault is now unlocked with new password)
	s.isUnlocked = true
	s.username = vault.Username
	s.passwordHint = newHint
	s.derivedKey = newVaultKey
	s.salt = newSalt
	s.vaultData = &vaultData

	return newRecoveryPhrase, nil
}
