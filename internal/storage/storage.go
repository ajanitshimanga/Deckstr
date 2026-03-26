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
	vaultVersion     = 1
	configDirName    = "OpenSmurfManager"
)

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
	isUnlocked  bool
	username    string // Current logged-in username
	derivedKey  []byte
	salt        []byte
	vaultData   *models.VaultData
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
		Salt:          crypto.EncodeBase64(s.salt),
		Nonce:         crypto.EncodeBase64(nonce),
		EncryptedData: crypto.EncodeBase64(ciphertext),
		CreatedAt:     time.Now(), // This will be overwritten if loading
		UpdatedAt:     time.Now(),
	}

	return s.saveVaultFile(vault)
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

	return &vault, nil
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
