package e2e

import (
	"os"
	"testing"

	"OpenSmurfManager/internal/accounts"
	"OpenSmurfManager/internal/storage"

	"OpenSmurfManager/internal/models"
)

// TestCustomNetwork_Roundtrip verifies that the new Custom-network fields
// (CustomNetwork, CustomGame) survive encryption + on-disk persistence + a
// fresh storage instance. Without this, a silent serialization regression
// could drop the user-provided labels without any crash, leaving accounts
// with an orphaned "custom" networkId and no label.
func TestCustomNetwork_Roundtrip(t *testing.T) {
	app := newTestApp(t)
	const (
		username = "test-user"
		password = "correct horse battery staple"
	)

	if err := app.Storage.CreateVault(username, password); err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}

	created, err := app.Accounts.Create(models.Account{
		DisplayName:   "Steam Smurf",
		Username:      "steam_user",
		Password:      "steam_pw",
		NetworkID:     "custom",
		CustomNetwork: "Steam",
		CustomGame:    "Counter-Strike 2",
		Tags:          []string{"main"},
		Notes:         "my CS2 account",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created account to have an ID")
	}

	// Lock + reopen from disk to prove persistence isn't just in-memory.
	app.Storage.Lock()

	store, acctSvc := reopenStorage(t, app.VaultPath)
	if err := store.Unlock(username, password); err != nil {
		t.Fatalf("Unlock after reopen failed: %v", err)
	}

	reloaded, err := acctSvc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID after reopen failed: %v", err)
	}

	if reloaded.NetworkID != "custom" {
		t.Errorf("NetworkID = %q, want %q", reloaded.NetworkID, "custom")
	}
	if reloaded.CustomNetwork != "Steam" {
		t.Errorf("CustomNetwork = %q, want %q", reloaded.CustomNetwork, "Steam")
	}
	if reloaded.CustomGame != "Counter-Strike 2" {
		t.Errorf("CustomGame = %q, want %q", reloaded.CustomGame, "Counter-Strike 2")
	}
	if reloaded.RiotID != "" {
		t.Errorf("RiotID should be empty for custom accounts, got %q", reloaded.RiotID)
	}
	if len(reloaded.Games) != 0 {
		t.Errorf("Games should be empty for custom accounts, got %v", reloaded.Games)
	}
}

// TestCustomNetwork_CoexistsWithRiot makes sure adding a Custom account
// alongside a Riot account doesn't scramble either — they're two discriminated
// cases that share one Account struct, and a naive shortcut in the serializer
// could leak fields across.
func TestCustomNetwork_CoexistsWithRiot(t *testing.T) {
	app := newTestApp(t)
	const (
		username = "test-user"
		password = "correct horse battery staple"
	)

	if err := app.Storage.CreateVault(username, password); err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}

	_, err := app.Accounts.Create(models.Account{
		DisplayName: "Riot Main",
		Username:    "rioter",
		Password:    "pw",
		NetworkID:   "riot",
		RiotID:      "Player#NA1",
		Region:      "na1",
		Games:       []string{"lol", "tft"},
	})
	if err != nil {
		t.Fatalf("Create riot account failed: %v", err)
	}
	_, err = app.Accounts.Create(models.Account{
		DisplayName:   "Epic Smurf",
		Username:      "epic_user",
		Password:      "pw2",
		NetworkID:     "custom",
		CustomNetwork: "Epic Games",
		CustomGame:    "Fortnite",
	})
	if err != nil {
		t.Fatalf("Create custom account failed: %v", err)
	}

	app.Storage.Lock()

	store, acctSvc := reopenStorage(t, app.VaultPath)
	if err := store.Unlock(username, password); err != nil {
		t.Fatalf("Unlock after reopen failed: %v", err)
	}

	all, err := acctSvc.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(all))
	}

	var riot, custom *models.Account
	for i := range all {
		switch all[i].NetworkID {
		case "riot":
			riot = &all[i]
		case "custom":
			custom = &all[i]
		}
	}
	if riot == nil || custom == nil {
		t.Fatalf("missing account: riot=%v, custom=%v", riot, custom)
	}

	// Neither account should have leaked fields from the other.
	if riot.CustomNetwork != "" || riot.CustomGame != "" {
		t.Errorf("Riot account has custom fields: CustomNetwork=%q CustomGame=%q",
			riot.CustomNetwork, riot.CustomGame)
	}
	if custom.RiotID != "" || custom.Region != "" || len(custom.Games) > 0 {
		t.Errorf("Custom account has Riot fields: RiotID=%q Region=%q Games=%v",
			custom.RiotID, custom.Region, custom.Games)
	}
}

// TestVault_TamperedFileFailsToUnlock verifies that if the encrypted payload
// on disk has been modified after write (bit flip, partial corruption, manual
// edit), the AES-GCM auth tag fails and the vault refuses to unlock rather
// than silently returning garbage. This is the guarantee the whole
// zero-knowledge model rests on.
func TestVault_TamperedFileFailsToUnlock(t *testing.T) {
	app := newTestApp(t)
	const (
		username = "test-user"
		password = "correct horse battery staple"
	)

	if err := app.Storage.CreateVault(username, password); err != nil {
		t.Fatalf("CreateVault failed: %v", err)
	}
	if _, err := app.Accounts.Create(models.Account{
		DisplayName: "Canary",
		Username:    "canary",
		Password:    "pw",
		NetworkID:   "riot",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	app.Storage.Lock()

	// Read the vault file, flip a byte somewhere inside the encrypted payload.
	raw, err := os.ReadFile(app.VaultPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(raw) < 200 {
		t.Fatalf("vault file smaller than expected (%d bytes) — test assumption invalid", len(raw))
	}
	// Flip a byte near the end — more likely to land in the ciphertext than
	// in the JSON header frame. GCM catches tampering anywhere in the
	// payload, so the exact position doesn't matter much.
	raw[len(raw)-20] ^= 0xFF
	if err := os.WriteFile(app.VaultPath, raw, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	store, _ := reopenStorage(t, app.VaultPath)
	err = store.Unlock(username, password)
	if err == nil {
		t.Fatal("tampered vault unlocked successfully — GCM auth not enforced")
	}
	// Keep the assertion loose; different errors can bubble up depending on
	// where the bit flip lands (JSON decode vs AEAD authentication).
	t.Logf("tampered vault rejected as expected: %v", err)
}

// Compile-time assertions so unused imports don't trip the build if any test
// is ever removed from this file.
var (
	_ = accounts.NewAccountService
	_ = storage.NewStorageServiceWithPath
)
