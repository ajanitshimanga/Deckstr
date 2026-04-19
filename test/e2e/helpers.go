// Package e2e contains end-to-end service integration tests that exercise
// the full wiring of storage, accounts, and providers using fakes for any
// external dependencies (game clients, network).
//
// These tests are deliberately at the "service composition" layer rather
// than the App struct itself, because App lives in package main and pulls
// in Wails runtime bindings. Running the same flows through the underlying
// services gives us regression coverage without the GUI dependency.
package e2e

import (
	"path/filepath"
	"testing"

	"OpenSmurfManager/internal/accounts"
	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/fake"
	"OpenSmurfManager/internal/storage"
)

// TestApp is a wired-up bundle of services using a temp vault path.
// Construct via newTestApp and it'll be cleaned up automatically via t.TempDir.
type TestApp struct {
	Storage   *storage.StorageService
	Accounts  *accounts.AccountService
	Providers *providers.Registry

	// VaultPath is the on-disk location of the temp vault, exposed so tests
	// can construct a fresh storage instance pointing at the same file
	// (useful for verifying that state truly persisted to disk vs. in-memory).
	VaultPath string

	// Provided fakes - tests can mutate these to control detection behavior
	RiotFake *fake.Provider
}

// newTestApp wires storage, accounts, and a providers registry with one
// fake "riot" provider registered. Each call gets a fresh temp vault.
//
// The pre-configured Riot fake is given a MatchFunc that simulates the real
// Riot provider's PUUID matching strategy, so tests can populate Account.PUUID
// and DetectedAccount.PUUID just like the real adapter does. Tests that
// need different behavior can overwrite RiotFake.MatchFunc.
func newTestApp(t *testing.T) *TestApp {
	t.Helper()

	vaultPath := filepath.Join(t.TempDir(), "vault.osm")
	store := storage.NewStorageServiceWithPath(vaultPath)
	acctSvc := accounts.NewAccountService(store)

	registry := providers.NewRegistry()
	riotFake := fake.New("riot", "Riot Games (Fake)")
	riotFake.MatchFunc = matchRiotByPUUID
	riotFake.UpdateFunc = updateRiotAccount
	registry.MustRegister(riotFake)

	return &TestApp{
		Storage:   store,
		Accounts:  acctSvc,
		Providers: registry,
		VaultPath: vaultPath,
		RiotFake:  riotFake,
	}
}

// matchRiotByPUUID mirrors the real Riot provider's matching: PUUID first,
// falling back to RiotID. Kept in the e2e helper (not the generic fake) so
// the fake itself stays platform-agnostic.
func matchRiotByPUUID(accts []models.Account, detected *providers.DetectedAccount) *models.Account {
	for i := range accts {
		if accts[i].NetworkID != "riot" {
			continue
		}
		if detected.PUUID != "" && accts[i].PUUID == detected.PUUID {
			return &accts[i]
		}
		if detected.RiotID != "" && accts[i].RiotID == detected.RiotID {
			return &accts[i]
		}
	}
	return nil
}

// updateRiotAccount mirrors what the real Riot adapter writes back onto a
// stored account when a session is detected.
func updateRiotAccount(account *models.Account, detected *providers.DetectedAccount) {
	if detected.PUUID != "" {
		account.PUUID = detected.PUUID
	}
	if detected.RiotID != "" {
		account.RiotID = detected.RiotID
	}
}

// reopenStorage returns a brand-new StorageService + AccountService pair
// pointing at the same vault file, simulating an app restart. Useful for
// asserting that state changes truly hit disk rather than just in-memory.
func reopenStorage(t *testing.T, vaultPath string) (*storage.StorageService, *accounts.AccountService) {
	t.Helper()
	store := storage.NewStorageServiceWithPath(vaultPath)
	return store, accounts.NewAccountService(store)
}
