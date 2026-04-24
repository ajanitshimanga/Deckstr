package storage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"OpenSmurfManager/internal/models"
)

// Helper to create a temporary storage service for testing
func newTestStorageService(t *testing.T) (*StorageService, string) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")

	// Use real storage service with real crypto
	service, err := NewStorageService()
	if err != nil {
		t.Fatalf("Failed to create storage service: %v", err)
	}
	service.vaultPath = vaultPath

	return service, tmpDir
}

// Test with real crypto service
func TestVaultCreationAndUnlock(t *testing.T) {
	// Create temp directory for vault
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Manually set up storage service with temp path
	vaultPath := filepath.Join(tmpDir, "test-vault.osm")

	// We need to test with the real implementation
	// Create a new storage service and override the path
	service, err := NewStorageService()
	if err != nil {
		t.Fatalf("NewStorageService() error = %v", err)
	}
	service.vaultPath = vaultPath

	username := "testuser"
	password := "testpassword123"

	// Vault should not exist initially
	if service.VaultExists() {
		t.Error("VaultExists() should be false before creation")
	}

	// Should not be unlocked
	if service.IsUnlocked() {
		t.Error("IsUnlocked() should be false before creation")
	}

	// Create vault
	err = service.CreateVault(username, password)
	if err != nil {
		t.Fatalf("CreateVault() error = %v", err)
	}

	// Vault should exist now
	if !service.VaultExists() {
		t.Error("VaultExists() should be true after creation")
	}

	// Should be unlocked after creation
	if !service.IsUnlocked() {
		t.Error("IsUnlocked() should be true after creation")
	}

	// Username should be set
	if service.GetUsername() != username {
		t.Errorf("GetUsername() = %v, want %v", service.GetUsername(), username)
	}

	// Lock the vault
	service.Lock()

	// Should be locked now
	if service.IsUnlocked() {
		t.Error("IsUnlocked() should be false after Lock()")
	}

	// Username should be cleared
	if service.GetUsername() != "" {
		t.Error("GetUsername() should be empty after Lock()")
	}

	// Unlock with correct credentials
	err = service.Unlock(username, password)
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	if !service.IsUnlocked() {
		t.Error("IsUnlocked() should be true after Unlock()")
	}
}

func TestUnlockWithWrongPassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	username := "testuser"
	correctPassword := "correctpassword"
	wrongPassword := "wrongpassword"

	// Create vault
	service.CreateVault(username, correctPassword)
	service.Lock()

	// Try to unlock with wrong password
	err = service.Unlock(username, wrongPassword)
	if err == nil {
		t.Error("Unlock() with wrong password should fail")
	}
	if err != ErrInvalidPassword {
		t.Errorf("Unlock() error = %v, want ErrInvalidPassword", err)
	}

	// Should still be locked
	if service.IsUnlocked() {
		t.Error("IsUnlocked() should be false after failed unlock")
	}
}

func TestUnlockWithWrongUsername(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	correctUsername := "correctuser"
	wrongUsername := "wronguser"
	password := "password123"

	// Create vault
	service.CreateVault(correctUsername, password)
	service.Lock()

	// Try to unlock with wrong username
	err = service.Unlock(wrongUsername, password)
	if err == nil {
		t.Error("Unlock() with wrong username should fail")
	}
	if err != ErrInvalidUsername {
		t.Errorf("Unlock() error = %v, want ErrInvalidUsername", err)
	}
}

func TestCannotAccessDataWhenLocked(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	service.CreateVault("user", "password")
	service.Lock()

	// GetVaultData should fail when locked
	_, err = service.GetVaultData()
	if err == nil {
		t.Error("GetVaultData() should fail when locked")
	}
	if err != ErrVaultLocked {
		t.Errorf("GetVaultData() error = %v, want ErrVaultLocked", err)
	}

	// UpdateVaultData should fail when locked
	err = service.UpdateVaultData(&models.VaultData{})
	if err == nil {
		t.Error("UpdateVaultData() should fail when locked")
	}
	if err != ErrVaultLocked {
		t.Errorf("UpdateVaultData() error = %v, want ErrVaultLocked", err)
	}

	// Save should fail when locked
	err = service.Save()
	if err == nil {
		t.Error("Save() should fail when locked")
	}
	if err != ErrVaultLocked {
		t.Errorf("Save() error = %v, want ErrVaultLocked", err)
	}
}

func TestCredentialsAreEncryptedInVaultFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	username := "secretuser"
	password := "supersecretpassword"

	service.CreateVault(username, password)

	// Add an account with sensitive credentials
	data, _ := service.GetVaultData()
	data.Accounts = append(data.Accounts, models.Account{
		ID:          "test-id",
		DisplayName: "Test Account",
		Username:    "game_username_123",
		Password:    "game_password_456",
		NetworkID:   "riot",
		Tags:        []string{"main"},
	})
	service.UpdateVaultData(data)
	service.Save()

	// Read the raw vault file
	rawData, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("Failed to read vault file: %v", err)
	}

	rawContent := string(rawData)

	// The vault file should NOT contain plaintext credentials
	sensitiveStrings := []string{
		"game_username_123",
		"game_password_456",
		"supersecretpassword",
	}

	for _, s := range sensitiveStrings {
		if strings.Contains(rawContent, s) {
			t.Errorf("Vault file contains plaintext sensitive data: %s", s)
		}
	}

	// The vault file SHOULD contain the username (it's not encrypted)
	// This is intentional for future cloud auth
	if !strings.Contains(rawContent, username) {
		t.Error("Vault file should contain the username (for auth purposes)")
	}

	// The vault file should contain encrypted data (base64)
	var vault models.Vault
	if err := json.Unmarshal(rawData, &vault); err != nil {
		t.Fatalf("Failed to parse vault JSON: %v", err)
	}

	if vault.EncryptedData == "" {
		t.Error("Vault should contain encrypted data")
	}
	if vault.Salt == "" {
		t.Error("Vault should contain salt")
	}
	if vault.Nonce == "" {
		t.Error("Vault should contain nonce")
	}
}

func TestCannotDecryptWithoutPassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	// Create vault with account
	service.CreateVault("user", "password123")
	data, _ := service.GetVaultData()
	data.Accounts = append(data.Accounts, models.Account{
		ID:       "acc1",
		Username: "secret_username",
		Password: "secret_password",
	})
	service.UpdateVaultData(data)
	service.Save()
	service.Lock()

	// Try to read vault file directly and parse encrypted data
	rawData, _ := os.ReadFile(vaultPath)
	var vault models.Vault
	json.Unmarshal(rawData, &vault)

	// The encrypted data should be base64 encoded
	// Attempting to decode and parse as JSON should fail
	// because it's encrypted, not plain JSON

	// This simulates an attacker trying to access the vault file directly
	// without knowing the password

	// The encryptedData is base64, but decoding it gives ciphertext
	// not readable JSON
	decoded, err := decodeBase64(vault.EncryptedData)
	if err != nil {
		// If decode fails, that's fine - data is protected
		return
	}

	// Try to parse as JSON - should fail because it's encrypted
	var vaultData models.VaultData
	err = json.Unmarshal(decoded, &vaultData)
	if err == nil {
		// If it parsed successfully, that would be a security issue
		// because it means data wasn't actually encrypted
		if len(vaultData.Accounts) > 0 && vaultData.Accounts[0].Username == "secret_username" {
			t.Error("SECURITY ISSUE: Was able to read account data without password!")
		}
	}
	// Expected: unmarshal fails because decoded data is ciphertext, not JSON
}

// Helper to decode base64 (same as in crypto package)
func decodeBase64(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

func TestVaultDataPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")

	// Create and populate vault with first service instance
	service1, _ := NewStorageService()
	service1.vaultPath = vaultPath

	username := "testuser"
	password := "testpass123"

	service1.CreateVault(username, password)
	data, _ := service1.GetVaultData()
	data.Accounts = append(data.Accounts, models.Account{
		ID:          "persistent-acc",
		DisplayName: "Persistent Account",
		Username:    "persist_user",
		Password:    "persist_pass",
		NetworkID:   "riot",
		Tags:        []string{"test"},
	})
	service1.UpdateVaultData(data)
	service1.Save()
	service1.Lock()

	// Create new service instance (simulating app restart)
	service2, _ := NewStorageService()
	service2.vaultPath = vaultPath

	// Unlock with credentials
	err = service2.Unlock(username, password)
	if err != nil {
		t.Fatalf("Failed to unlock after restart: %v", err)
	}

	// Data should be preserved
	data2, err := service2.GetVaultData()
	if err != nil {
		t.Fatalf("Failed to get vault data: %v", err)
	}

	if len(data2.Accounts) != 1 {
		t.Fatalf("Expected 1 account, got %d", len(data2.Accounts))
	}

	acc := data2.Accounts[0]
	if acc.ID != "persistent-acc" {
		t.Errorf("Account ID = %v, want persistent-acc", acc.ID)
	}
	if acc.Username != "persist_user" {
		t.Errorf("Account Username = %v, want persist_user", acc.Username)
	}
	if acc.Password != "persist_pass" {
		t.Errorf("Account Password = %v, want persist_pass", acc.Password)
	}
}

func TestGetStoredUsername(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	expectedUsername := "myusername"

	service.CreateVault(expectedUsername, "password")
	service.Lock()

	// Should be able to get stored username without unlocking
	storedUsername, err := service.GetStoredUsername()
	if err != nil {
		t.Fatalf("GetStoredUsername() error = %v", err)
	}

	if storedUsername != expectedUsername {
		t.Errorf("GetStoredUsername() = %v, want %v", storedUsername, expectedUsername)
	}
}

func TestCannotCreateDuplicateVault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	// Create first vault
	err = service.CreateVault("user1", "pass1")
	if err != nil {
		t.Fatalf("First CreateVault() error = %v", err)
	}

	service.Lock()

	// Try to create another vault - should fail
	err = service.CreateVault("user2", "pass2")
	if err == nil {
		t.Error("CreateVault() should fail when vault already exists")
	}
	if err != ErrVaultExists {
		t.Errorf("CreateVault() error = %v, want ErrVaultExists", err)
	}
}

// ============================================================================
// Password Hint Tests
// ============================================================================

func TestCreateVaultWithHint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	username := "testuser"
	password := "testpassword123"
	hint := "My favorite pet's name"

	// Create vault with hint
	err = service.CreateVaultWithHint(username, password, hint)
	if err != nil {
		t.Fatalf("CreateVaultWithHint() error = %v", err)
	}

	// Vault should be created and unlocked
	if !service.VaultExists() {
		t.Error("VaultExists() should be true after creation")
	}
	if !service.IsUnlocked() {
		t.Error("IsUnlocked() should be true after creation")
	}
}

func TestGetPasswordHintWithoutUnlocking(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	username := "testuser"
	password := "testpassword123"
	expectedHint := "My favorite color"

	// Create vault with hint
	service.CreateVaultWithHint(username, password, expectedHint)
	service.Lock()

	// Should be able to get hint without unlocking
	hint, err := service.GetPasswordHint()
	if err != nil {
		t.Fatalf("GetPasswordHint() error = %v", err)
	}

	if hint != expectedHint {
		t.Errorf("GetPasswordHint() = %v, want %v", hint, expectedHint)
	}
}

func TestPasswordHintStoredInVaultFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	hint := "The street I grew up on"
	service.CreateVaultWithHint("user", "password", hint)

	// Read raw vault file
	rawData, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("Failed to read vault file: %v", err)
	}

	// Hint should be visible in the vault file (it's intentionally not encrypted)
	if !strings.Contains(string(rawData), hint) {
		t.Error("Password hint should be stored in vault file (unencrypted for display on lock screen)")
	}

	// Parse the vault to verify structure
	var vault models.Vault
	if err := json.Unmarshal(rawData, &vault); err != nil {
		t.Fatalf("Failed to parse vault: %v", err)
	}

	if vault.PasswordHint != hint {
		t.Errorf("Vault.PasswordHint = %v, want %v", vault.PasswordHint, hint)
	}
}

func TestCreateVaultWithEmptyHint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	// Create vault with empty hint
	err = service.CreateVaultWithHint("user", "password", "")
	if err != nil {
		t.Fatalf("CreateVaultWithHint() with empty hint should succeed, error = %v", err)
	}

	service.Lock()

	hint, err := service.GetPasswordHint()
	if err != nil {
		t.Fatalf("GetPasswordHint() error = %v", err)
	}

	if hint != "" {
		t.Errorf("GetPasswordHint() = %v, want empty string", hint)
	}
}

func TestGetPasswordHintWhenNoVault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	// No vault exists
	_, err = service.GetPasswordHint()
	if err == nil {
		t.Error("GetPasswordHint() should fail when no vault exists")
	}
	if err != ErrVaultNotFound {
		t.Errorf("GetPasswordHint() error = %v, want ErrVaultNotFound", err)
	}
}

// ============================================================================
// Password Change Tests
// ============================================================================

func TestChangePassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	username := "testuser"
	oldPassword := "oldpassword123"
	newPassword := "newpassword456"

	// Create vault and add some data
	service.CreateVault(username, oldPassword)
	data, _ := service.GetVaultData()
	data.Accounts = append(data.Accounts, models.Account{
		ID:       "acc1",
		Username: "myaccount",
		Password: "mypassword",
	})
	service.UpdateVaultData(data)
	service.Save()

	// Change password
	recoveryPhrase, err := service.ChangePassword(oldPassword, newPassword)
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if recoveryPhrase == "" {
		t.Error("ChangePassword() should return a recovery phrase")
	}

	// Should still be unlocked after password change
	if !service.IsUnlocked() {
		t.Error("IsUnlocked() should be true after password change")
	}

	// Lock and try to unlock with new password
	service.Lock()
	err = service.Unlock(username, newPassword)
	if err != nil {
		t.Fatalf("Unlock() with new password error = %v", err)
	}

	// Data should be preserved
	data, _ = service.GetVaultData()
	if len(data.Accounts) != 1 {
		t.Fatalf("Expected 1 account, got %d", len(data.Accounts))
	}
	if data.Accounts[0].Username != "myaccount" {
		t.Errorf("Account username = %v, want myaccount", data.Accounts[0].Username)
	}
}

func TestChangePasswordFailsWithWrongCurrentPassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	correctPassword := "correctpassword"
	wrongPassword := "wrongpassword"
	newPassword := "newpassword"

	service.CreateVault("user", correctPassword)

	// Try to change password with wrong current password
	_, err = service.ChangePassword(wrongPassword, newPassword)
	if err == nil {
		t.Error("ChangePassword() with wrong current password should fail")
	}
	if err != ErrInvalidPassword {
		t.Errorf("ChangePassword() error = %v, want ErrInvalidPassword", err)
	}

	// Original password should still work
	service.Lock()
	err = service.Unlock("user", correctPassword)
	if err != nil {
		t.Errorf("Original password should still work after failed change, error = %v", err)
	}
}

func TestChangePasswordFailsWhenLocked(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	password := "password123"
	service.CreateVault("user", password)
	service.Lock()

	// Try to change password when locked
	_, err = service.ChangePassword(password, "newpassword")
	if err == nil {
		t.Error("ChangePassword() when locked should fail")
	}
	if err != ErrVaultLocked {
		t.Errorf("ChangePassword() error = %v, want ErrVaultLocked", err)
	}
}

func TestCannotUnlockWithOldPasswordAfterChange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	username := "testuser"
	oldPassword := "oldpassword"
	newPassword := "newpassword"

	service.CreateVault(username, oldPassword)
	_, _ = service.ChangePassword(oldPassword, newPassword)
	service.Lock()

	// Old password should NOT work
	err = service.Unlock(username, oldPassword)
	if err == nil {
		t.Error("Unlock() with old password should fail after password change")
	}
	if err != ErrInvalidPassword {
		t.Errorf("Unlock() error = %v, want ErrInvalidPassword", err)
	}
}

func TestChangePasswordGeneratesNewSalt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	service.CreateVault("user", "oldpass")

	// Read original vault
	originalRaw, _ := os.ReadFile(vaultPath)
	var originalVault models.Vault
	json.Unmarshal(originalRaw, &originalVault)

	// Change password
	service.ChangePassword("oldpass", "newpass")

	// Read new vault
	newRaw, _ := os.ReadFile(vaultPath)
	var newVault models.Vault
	json.Unmarshal(newRaw, &newVault)

	// Salt should be different (new key derivation)
	if originalVault.Salt == newVault.Salt {
		t.Error("Salt should change when password is changed")
	}

	// Nonce should be different
	if originalVault.Nonce == newVault.Nonce {
		t.Error("Nonce should change when password is changed")
	}

	// Encrypted data should be different
	if originalVault.EncryptedData == newVault.EncryptedData {
		t.Error("EncryptedData should change when password is changed")
	}
}

func TestChangePasswordPreservesHint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	hint := "My first car"
	service.CreateVaultWithHint("user", "oldpass", hint)
	service.ChangePassword("oldpass", "newpass")

	// Hint should still be accessible
	retrievedHint, err := service.GetPasswordHint()
	if err != nil {
		t.Fatalf("GetPasswordHint() error = %v", err)
	}
	if retrievedHint != hint {
		t.Errorf("Hint = %v, want %v (should be preserved after password change)", retrievedHint, hint)
	}
}

// ============================================================================
// Update Password Hint Tests (for legacy users)
// ============================================================================

func TestUpdatePasswordHint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	// Create vault WITHOUT hint (simulating legacy user)
	service.CreateVault("user", "password")

	// Verify no hint initially
	hint, _ := service.GetPasswordHint()
	if hint != "" {
		t.Errorf("Initial hint should be empty, got %v", hint)
	}

	// Update hint
	newHint := "My childhood nickname"
	err = service.UpdatePasswordHint(newHint)
	if err != nil {
		t.Fatalf("UpdatePasswordHint() error = %v", err)
	}

	// Verify hint is updated
	hint, err = service.GetPasswordHint()
	if err != nil {
		t.Fatalf("GetPasswordHint() error = %v", err)
	}
	if hint != newHint {
		t.Errorf("Hint = %v, want %v", hint, newHint)
	}
}

func TestUpdatePasswordHintFailsWhenLocked(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	service.CreateVault("user", "password")
	service.Lock()

	// Should fail when locked
	err = service.UpdatePasswordHint("new hint")
	if err == nil {
		t.Error("UpdatePasswordHint() when locked should fail")
	}
	if err != ErrVaultLocked {
		t.Errorf("UpdatePasswordHint() error = %v, want ErrVaultLocked", err)
	}
}

func TestUpdatePasswordHintPersistsAcrossRestart(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")

	// First session - create vault and update hint
	service1, _ := NewStorageService()
	service1.vaultPath = vaultPath
	service1.CreateVault("user", "password")
	service1.UpdatePasswordHint("Remember this!")
	service1.Lock()

	// Second session - simulate app restart
	service2, _ := NewStorageService()
	service2.vaultPath = vaultPath

	// Hint should be readable without unlocking
	hint, err := service2.GetPasswordHint()
	if err != nil {
		t.Fatalf("GetPasswordHint() error = %v", err)
	}
	if hint != "Remember this!" {
		t.Errorf("Hint = %v, want 'Remember this!'", hint)
	}
}

func TestClearPasswordHint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vaultPath := filepath.Join(tmpDir, "test-vault.osm")
	service, _ := NewStorageService()
	service.vaultPath = vaultPath

	// Create with hint
	service.CreateVaultWithHint("user", "password", "Original hint")

	// Clear hint by setting to empty
	err = service.UpdatePasswordHint("")
	if err != nil {
		t.Fatalf("UpdatePasswordHint('') error = %v", err)
	}

	// Verify hint is cleared
	hint, _ := service.GetPasswordHint()
	if hint != "" {
		t.Errorf("Hint should be empty after clearing, got %v", hint)
	}
}

// ============================================================================
// Recovery scheme v2 migration tests. These pin the core security invariant
// of the v2 scheme (verify ≠ encrypt on disk), the v1→v2 migration contract,
// and the forward-compatibility fence for unsupported future versions.
// ============================================================================

// TestRecoveryHashIsNotEncryptionKey pins THE critical invariant the v2
// scheme exists to enforce: the RecoveryPhraseHash stored in vault.osm
// cannot be used as an AES-GCM key to decrypt the stored EncryptedVaultKey.
// If this test ever starts passing decryption, the v1 bug is back.
func TestRecoveryHashIsNotEncryptionKey(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	_, err := service.CreateVaultWithRecoveryPhrase("user", "strong-password", "")
	if err != nil {
		t.Fatalf("CreateVaultWithRecoveryPhrase: %v", err)
	}

	// Read the raw vault file the same way an attacker with FS access would.
	raw, err := os.ReadFile(service.vaultPath)
	if err != nil {
		t.Fatalf("read vault: %v", err)
	}
	var vault models.Vault
	if err := json.Unmarshal(raw, &vault); err != nil {
		t.Fatalf("parse vault: %v", err)
	}
	if vault.Version != 2 {
		t.Fatalf("fresh vaults must be stamped v2, got v%d", vault.Version)
	}

	verifyHash, _ := base64.StdEncoding.DecodeString(vault.RecoveryPhraseHash)
	encryptedVaultKey, _ := base64.StdEncoding.DecodeString(vault.EncryptedVaultKey)
	nonce, _ := base64.StdEncoding.DecodeString(vault.RecoveryKeyNonce)

	// The v1 bug was that verifyHash could be used directly as the AES-GCM
	// key. In v2, HKDF guarantees this fails.
	_, decryptErr := service.crypto.Decrypt(encryptedVaultKey, verifyHash, nonce)
	if decryptErr == nil {
		t.Fatal("CRITICAL: RecoveryPhraseHash decrypted EncryptedVaultKey — v1 bug regression")
	}
}

// TestUnlockV1VaultMigratesInPlace — construct a v1 vault using the legacy
// derivation (the bug), unlock with master password, and assert that the
// vault is silently rewritten to v2 with a freshly-rotated recovery phrase.
func TestUnlockV1VaultMigratesInPlace(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	// Seed a v1-style vault on disk by reaching below the public API.
	seedV1Vault(t, service, "user", "pw", "hint-text")

	if err := service.Unlock("user", "pw"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Post-unlock the vault on disk must be v2.
	raw, _ := os.ReadFile(service.vaultPath)
	var vault models.Vault
	_ = json.Unmarshal(raw, &vault)
	if vault.Version != 2 {
		t.Fatalf("expected vault to be migrated to v2, got v%d", vault.Version)
	}

	// Rotated phrase must be available via the sidecar API.
	phrase, ok := service.ConsumePendingRecoveryRotation()
	if !ok || phrase == "" {
		t.Fatal("expected a pending rotated phrase after v1 unlock")
	}

	// Sidecar is single-use — second call returns empty.
	_, ok2 := service.ConsumePendingRecoveryRotation()
	if ok2 {
		t.Fatal("ConsumePendingRecoveryRotation must be single-use")
	}

	// Hint, username, and account data all survived the migration.
	if service.GetUsername() != "user" {
		t.Errorf("username not preserved")
	}
	if vault.PasswordHint != "hint-text" {
		t.Errorf("hint not preserved, got %q", vault.PasswordHint)
	}
}

// TestUnlockV1VaultWrongPasswordDoesNotMigrate — a wrong password on a v1
// vault must not trigger rotation. The vault stays v1, the file on disk
// stays untouched, and the next correct unlock gets its chance to migrate.
func TestUnlockV1VaultWrongPasswordDoesNotMigrate(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	seedV1Vault(t, service, "user", "real-password", "")
	before, _ := os.ReadFile(service.vaultPath)

	if err := service.Unlock("user", "WRONG-password"); err == nil {
		t.Fatal("Unlock with wrong password must fail")
	}

	after, _ := os.ReadFile(service.vaultPath)
	if !bytes.Equal(before, after) {
		t.Fatal("vault file was modified after failed unlock — migration must not run on bad password")
	}
}

// TestV2VaultIdempotentOnUnlock — a vault already on v2 must not be
// rewritten on unlock (no migration, no new phrase).
func TestV2VaultIdempotentOnUnlock(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	_, err := service.CreateVaultWithRecoveryPhrase("user", "pw", "")
	if err != nil {
		t.Fatalf("CreateVaultWithRecoveryPhrase: %v", err)
	}
	_, _ = service.ConsumePendingRecoveryRotation() // drain any pending from create (there shouldn't be one)
	service.Lock()

	before, _ := os.ReadFile(service.vaultPath)

	if err := service.Unlock("user", "pw"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	_, ok := service.ConsumePendingRecoveryRotation()
	if ok {
		t.Fatal("v2 unlock must not produce a rotated phrase")
	}

	after, _ := os.ReadFile(service.vaultPath)
	if !bytes.Equal(before, after) {
		t.Fatal("v2 vault was rewritten on plain unlock")
	}
}

// TestUnlockRefusesForwardVersions — a vault from a future Deckstr build
// must refuse to open rather than try to decrypt under wrong assumptions.
func TestUnlockRefusesForwardVersions(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	// Start from a valid v2 vault, then bump Version to 99 and re-save.
	_, err := service.CreateVaultWithRecoveryPhrase("user", "pw", "")
	if err != nil {
		t.Fatalf("CreateVaultWithRecoveryPhrase: %v", err)
	}
	service.Lock()
	raw, _ := os.ReadFile(service.vaultPath)
	var vault models.Vault
	_ = json.Unmarshal(raw, &vault)
	vault.Version = 99
	remarshaled, _ := json.MarshalIndent(vault, "", "  ")
	_ = os.WriteFile(service.vaultPath, remarshaled, 0600)

	err = service.Unlock("user", "pw")
	if err == nil {
		t.Fatal("Unlock must refuse v99 vault")
	}
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("expected ErrUnsupportedVersion, got %v", err)
	}
}

// TestMigrateVaultResetsFlagForV2 pins that loading a v2 vault clears
// any stale needsRecoveryRotation flag left over from a prior unlock.
// Without this, a wrong-password attempt on a v1 vault could leave the
// flag true, and if the on-disk vault were swapped to v2 before the
// next attempt, rotation would incorrectly fire on the v2 vault.
func TestMigrateVaultResetsFlagForV2(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	// Create a v2 vault on disk, then lock.
	_, err := service.CreateVaultWithRecoveryPhrase("user", "pw", "")
	if err != nil {
		t.Fatalf("create v2 vault: %v", err)
	}
	_, _ = service.ConsumePendingRecoveryRotation()
	service.Lock()

	// Simulate a stale flag left over from a prior failed v1 unlock attempt
	// (which, before this fix, migrateVault had no path to clear).
	service.needsRecoveryRotation = true

	// Load the v2 vault. migrateVault must clear the stale flag.
	v, err := service.loadVaultFile()
	if err != nil {
		t.Fatalf("loadVaultFile: %v", err)
	}
	if v.Version != 2 {
		t.Fatalf("expected on-disk v2, got v%d", v.Version)
	}
	if service.needsRecoveryRotation {
		t.Error("migrateVault must clear the rotation flag when loading a v2 vault")
	}
}

// TestResetWithPhraseOnV1Vault — the forgot-password path must work for
// users who never unlocked after the v1.3.1 update. Reset with their v1
// phrase, land on a v2 vault with a new phrase.
func TestResetWithPhraseOnV1Vault(t *testing.T) {
	service, tmpDir := newTestStorageService(t)
	defer os.RemoveAll(tmpDir)

	seedPhrase := seedV1VaultReturningPhrase(t, service, "user", "original-pw")

	newPhrase, err := service.ResetPasswordWithRecoveryPhrase(seedPhrase, "new-pw", "")
	if err != nil {
		t.Fatalf("ResetPasswordWithRecoveryPhrase on v1 vault: %v", err)
	}
	if newPhrase == "" || newPhrase == seedPhrase {
		t.Error("reset must produce a distinct new phrase")
	}

	// Vault is now v2 and new password unlocks it.
	raw, _ := os.ReadFile(service.vaultPath)
	var vault models.Vault
	_ = json.Unmarshal(raw, &vault)
	if vault.Version != 2 {
		t.Errorf("expected vault to be v2 after reset, got v%d", vault.Version)
	}

	service.Lock()
	if err := service.Unlock("user", "new-pw"); err != nil {
		t.Errorf("unlock with new password after v1-reset failed: %v", err)
	}
}

// seedV1Vault writes a vault.osm to disk that uses the broken v1 recovery
// scheme (verifyHash == encryptKey). It bypasses the public API, which
// always produces v2 vaults post-patch. Returns the cleartext of the seeded
// phrase for tests that need to exercise the phrase path.
func seedV1Vault(t *testing.T, s *StorageService, username, password, hint string) {
	t.Helper()
	_ = seedV1VaultReturningPhrase(t, s, username, password)
	// Patch in the hint we wanted (seedV1VaultReturningPhrase doesn't take one).
	if hint != "" {
		raw, _ := os.ReadFile(s.vaultPath)
		var v models.Vault
		_ = json.Unmarshal(raw, &v)
		v.PasswordHint = hint
		out, _ := json.MarshalIndent(v, "", "  ")
		_ = os.WriteFile(s.vaultPath, out, 0600)
	}
}

func seedV1VaultReturningPhrase(t *testing.T, s *StorageService, username, password string) string {
	t.Helper()

	phrase, err := s.crypto.GenerateRecoveryPhrase()
	if err != nil {
		t.Fatalf("seed phrase: %v", err)
	}
	recoverySalt, _ := s.crypto.GenerateSalt()
	// v1 bug: verify hash AND encrypt key are the same Argon2id output.
	recoveryHash := s.crypto.HashRecoveryPhrase(phrase, recoverySalt)
	recoveryKey := recoveryHash

	vaultData := models.NewVaultData()
	plaintext, _ := json.Marshal(vaultData)
	salt, nonce, ciphertext, err := s.crypto.EncryptWithPassword(plaintext, password)
	if err != nil {
		t.Fatalf("seed encrypt: %v", err)
	}
	vaultKey := s.crypto.DeriveKey(password, salt)
	recoveryKeyNonce, _ := s.crypto.GenerateNonce()
	encryptedVaultKey, err := s.crypto.Encrypt(vaultKey, recoveryKey, recoveryKeyNonce)
	if err != nil {
		t.Fatalf("seed wrap vault key: %v", err)
	}

	v := models.Vault{
		Version:            1, // the broken scheme
		Username:           username,
		Salt:               base64.StdEncoding.EncodeToString(salt),
		Nonce:              base64.StdEncoding.EncodeToString(nonce),
		EncryptedData:      base64.StdEncoding.EncodeToString(ciphertext),
		RecoveryPhraseHash: base64.StdEncoding.EncodeToString(recoveryHash),
		RecoveryPhraseSalt: base64.StdEncoding.EncodeToString(recoverySalt),
		RecoveryKeyNonce:   base64.StdEncoding.EncodeToString(recoveryKeyNonce),
		EncryptedVaultKey:  base64.StdEncoding.EncodeToString(encryptedVaultKey),
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(s.vaultPath, out, 0600); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	return phrase
}
