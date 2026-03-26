package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"OpenSmurfManager/internal/crypto"
	"OpenSmurfManager/internal/models"
)

const (
	vaultFileName    = "vault.osm" // OpenSmurfManager vault file
	vaultVersion     = 1           // Increment when making breaking changes to vault structure
	configDirName    = "OpenSmurfManager"
)

// Migration notes:
// - v1: Initial release (v1.0.0)
// - v1 additions (v1.1.0): Added passwordHint field (non-breaking, defaults to empty)

var (
	ErrVaultNotFound     = errors.New("vault not found")
	ErrVaultLocked       = errors.New("vault is locked")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrInvalidUsername   = errors.New("invalid username")
	ErrVaultExists       = errors.New("vault already exists")
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
}

// NewStorageService creates a new storage service
func NewStorageService() (*StorageService, error) {
	vaultPath, err := getVaultPath()
	if err != nil {
		return nil, err
	}

	return &StorageService{
		crypto:    crypto.NewCryptoService(),
		vaultPath: vaultPath,
	}, nil
}

// getVaultPath returns the path to the vault file
func getVaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}

	appDir := filepath.Join(configDir, configDirName)
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create app directory: %w", err)
	}

	return filepath.Join(appDir, vaultFileName), nil
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
	vault := models.Vault{
		Version:       vaultVersion,
		Username:      username,
		PasswordHint:  hint,
		Salt:          crypto.EncodeBase64(salt),
		Nonce:         crypto.EncodeBase64(nonce),
		EncryptedData: crypto.EncodeBase64(ciphertext),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
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

	// Store unlocked state
	s.isUnlocked = true
	s.username = username
	s.passwordHint = vault.PasswordHint
	s.derivedKey = s.crypto.DeriveKey(masterPassword, salt)
	s.salt = salt
	s.vaultData = &vaultData

	return nil
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

	// Update vault structure with new hint
	vault := models.Vault{
		Version:       vaultVersion,
		Username:      s.username,
		PasswordHint:  hint,
		Salt:          crypto.EncodeBase64(s.salt),
		Nonce:         crypto.EncodeBase64(nonce),
		EncryptedData: crypto.EncodeBase64(ciphertext),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
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

	// Update vault structure
	vault := models.Vault{
		Version:       vaultVersion,
		Username:      s.username,
		PasswordHint:  s.passwordHint,
		Salt:          crypto.EncodeBase64(s.salt),
		Nonce:         crypto.EncodeBase64(nonce),
		EncryptedData: crypto.EncodeBase64(ciphertext),
		CreatedAt:     time.Now(), // This will be overwritten if loading
		UpdatedAt:     time.Now(),
	}

	return s.saveVaultFile(vault)
}

// ChangePassword re-encrypts the vault with a new password
// The user must be unlocked and provide the correct current password
func (s *StorageService) ChangePassword(currentPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isUnlocked {
		return ErrVaultLocked
	}

	// Verify current password by attempting to derive key and compare
	// We do this by re-deriving the key and checking if it matches
	testKey := s.crypto.DeriveKey(currentPassword, s.salt)
	defer crypto.ClearBytes(testKey)

	// Compare with stored derived key
	if len(testKey) != len(s.derivedKey) {
		return ErrInvalidPassword
	}
	for i := range testKey {
		if testKey[i] != s.derivedKey[i] {
			return ErrInvalidPassword
		}
	}

	// Generate new salt and nonce for the new password
	newSalt, err := s.crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate new salt: %w", err)
	}

	newNonce, err := s.crypto.GenerateNonce()
	if err != nil {
		return fmt.Errorf("failed to generate new nonce: %w", err)
	}

	// Derive new key from new password
	newKey := s.crypto.DeriveKey(newPassword, newSalt)

	// Serialize vault data
	plaintext, err := json.Marshal(s.vaultData)
	if err != nil {
		return fmt.Errorf("failed to serialize vault data: %w", err)
	}

	// Encrypt with new key
	ciphertext, err := s.crypto.Encrypt(plaintext, newKey, newNonce)
	if err != nil {
		return fmt.Errorf("failed to encrypt vault: %w", err)
	}

	// Create new vault structure
	vault := models.Vault{
		Version:       vaultVersion,
		Username:      s.username,
		PasswordHint:  s.passwordHint,
		Salt:          crypto.EncodeBase64(newSalt),
		Nonce:         crypto.EncodeBase64(newNonce),
		EncryptedData: crypto.EncodeBase64(ciphertext),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Save to file
	if err := s.saveVaultFile(vault); err != nil {
		return err
	}

	// Clear old key and update state
	crypto.ClearBytes(s.derivedKey)
	s.derivedKey = newKey
	s.salt = newSalt

	return nil
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

// migrateVault handles data migrations between vault versions
// This function should be idempotent and handle all version upgrades
func (s *StorageService) migrateVault(vault *models.Vault) error {
	// Current version - no migrations needed yet
	// When adding breaking changes in future versions, add migration logic here:
	//
	// if vault.Version < 2 {
	//     // Migrate from v1 to v2
	//     vault.NewField = defaultValue
	//     vault.Version = 2
	//     // Note: Caller should save after successful unlock to persist migration
	// }

	return nil
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
