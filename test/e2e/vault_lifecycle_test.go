package e2e

import (
	"testing"

	"OpenSmurfManager/internal/models"
)

// TestVaultLifecycle_CreateAddLockUnlockReload exercises the most fundamental
// user journey: a new user creates a vault, adds an account, locks it,
// unlocks again, and verifies their data persists.
//
// This is the canary test - if this fails, basic vault functionality is broken.
func TestVaultLifecycle_CreateAddLockUnlockReload(t *testing.T) {
	app := newTestApp(t)
	const (
		username = "test-user"
		password = "correct horse battery staple"
	)

	// 1. Create vault
	if err := app.Storage.CreateVault(username, password); err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}
	if !app.Storage.VaultExists() {
		t.Fatal("vault should exist after creation")
	}
	if !app.Storage.IsUnlocked() {
		t.Fatal("vault should be unlocked after creation")
	}

	// 2. Add an account
	created, err := app.Accounts.Create(models.Account{
		DisplayName: "Main",
		Username:    "rioter",
		Password:    "secret",
		NetworkID:   "riot",
		RiotID:      "Player#NA1",
	})
	if err != nil {
		t.Fatalf("Create account failed: %v", err)
	}
	if created.ID == "" {
		t.Error("created account must have an ID assigned")
	}

	// 3. Lock vault and verify state cleared
	app.Storage.Lock()
	if app.Storage.IsUnlocked() {
		t.Fatal("vault should be locked after Lock()")
	}

	// 4. Reopen from disk (simulates app restart) and unlock
	freshStorage, freshAccounts := reopenStorage(t, app.VaultPath)
	if err := freshStorage.Unlock(username, password); err != nil {
		t.Fatalf("Unlock with correct password failed: %v", err)
	}

	// 5. Verify accounts persisted to disk
	all, err := freshAccounts.GetAll()
	if err != nil {
		t.Fatalf("GetAll after unlock failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 account after reload, got %d", len(all))
	}
	if all[0].Username != "rioter" {
		t.Errorf("account.Username = %q, want %q", all[0].Username, "rioter")
	}
	if all[0].RiotID != "Player#NA1" {
		t.Errorf("account.RiotID = %q, want %q", all[0].RiotID, "Player#NA1")
	}
}

// TestVaultLifecycle_WrongPasswordRejected ensures a wrong password cannot
// unlock the vault. This is a critical security regression test.
func TestVaultLifecycle_WrongPasswordRejected(t *testing.T) {
	app := newTestApp(t)
	if err := app.Storage.CreateVault("user", "right-password"); err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}
	app.Storage.Lock()

	if err := app.Storage.Unlock("user", "wrong-password"); err == nil {
		t.Fatal("Unlock with wrong password must return an error")
	}
	if app.Storage.IsUnlocked() {
		t.Fatal("vault must remain locked after failed unlock")
	}
}

// TestVaultLifecycle_AccountCRUD covers create, read, update, delete on
// an unlocked vault. Each operation should round-trip through encryption.
func TestVaultLifecycle_AccountCRUD(t *testing.T) {
	app := newTestApp(t)
	if err := app.Storage.CreateVault("user", "pw"); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}

	created, err := app.Accounts.Create(models.Account{
		Username:  "alpha",
		Password:  "p1",
		NetworkID: "riot",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update
	created.DisplayName = "Updated Display"
	updated, err := app.Accounts.Update(*created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DisplayName != "Updated Display" {
		t.Errorf("Update did not persist DisplayName")
	}

	// Read back
	got, err := app.Accounts.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DisplayName != "Updated Display" {
		t.Errorf("GetByID returned stale data: %q", got.DisplayName)
	}

	// Delete
	if err := app.Accounts.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := app.Accounts.GetByID(created.ID); err == nil {
		t.Fatal("GetByID after Delete should return error")
	}
}

// TestVaultLifecycle_PasswordChange verifies a user can change their master
// password and re-unlock with the new one. Critically, this uses a brand-new
// StorageService after the change to prove the new password is durable on
// disk, not just cached in memory.
func TestVaultLifecycle_PasswordChange(t *testing.T) {
	app := newTestApp(t)
	if err := app.Storage.CreateVault("user", "old-pw"); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}

	// Add an account so we can verify data preservation
	if _, err := app.Accounts.Create(models.Account{
		Username: "preserved", NetworkID: "riot",
	}); err != nil {
		t.Fatalf("Create account: %v", err)
	}

	if _, err := app.Storage.ChangePassword("old-pw", "new-pw"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Simulate an app restart by spinning up a fresh storage instance pointing
	// at the same vault file. This catches bugs where ChangePassword updates
	// in-memory state but doesn't fully re-encrypt the on-disk vault.
	freshStorage, freshAccounts := reopenStorage(t, app.VaultPath)

	// Old password must not work against the on-disk vault
	if err := freshStorage.Unlock("user", "old-pw"); err == nil {
		t.Error("old password must not work after change (on-disk verification)")
	}

	// New password works
	if err := freshStorage.Unlock("user", "new-pw"); err != nil {
		t.Fatalf("new password failed against fresh storage: %v", err)
	}

	// Data still there after re-encryption
	all, err := freshAccounts.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 1 || all[0].Username != "preserved" {
		t.Errorf("account data lost across password change: %v", all)
	}
}
