package e2e

import (
	"context"
	"testing"
	"time"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
)

// TestDetection_NoClientRunning verifies the registry returns (nil, nil)
// when no provider has an active session - this is the steady state of
// polling when the user doesn't have any clients open.
func TestDetection_NoClientRunning(t *testing.T) {
	app := newTestApp(t)
	app.RiotFake.DetectErr = &providers.DetectionError{
		Code:    providers.ErrCodeClientOffline,
		Message: "no client",
	}

	got, err := app.Providers.DetectAny(context.Background())
	if err != nil {
		t.Fatalf("DetectAny should swallow DetectionError, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil detection, got %#v", got)
	}
}

// TestDetection_MatchExistingAccount simulates the happy path: a user
// has stored their account, the client is running, and we successfully
// match + update ranks. This is the core user-visible value of the app.
func TestDetection_MatchExistingAccount(t *testing.T) {
	app := newTestApp(t)
	if err := app.Storage.CreateVault("user", "pw"); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}

	// Store an account with a known PUUID
	stored, err := app.Accounts.Create(models.Account{
		Username:  "smurfer",
		NetworkID: "riot",
		RiotID:    "Smurf#NA1",
		PUUID:     "puuid-12345",
	})
	if err != nil {
		t.Fatalf("Create account: %v", err)
	}

	// Configure fake provider to "detect" that exact account
	app.RiotFake.ClientRunning = true
	app.RiotFake.Detected = &providers.DetectedAccount{
		NetworkID:   "riot",
		DisplayName: "Smurf#NA1",
		UniqueID:    "puuid-12345",
		PUUID:       "puuid-12345",
		RiotID:      "Smurf#NA1",
		DetectedAt:  time.Now(),
		Ranks: []models.CachedRank{
			{GameID: "lol", QueueType: "RANKED_SOLO_5x5", Tier: "DIAMOND", Division: "II", LP: 50},
		},
	}

	// Detect
	detected, err := app.Providers.DetectAny(context.Background())
	if err != nil {
		t.Fatalf("DetectAny: %v", err)
	}
	if detected == nil {
		t.Fatal("expected to detect signed-in account")
	}

	// Match against stored accounts
	allAccounts, err := app.Accounts.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	matched := app.Providers.MatchAccount(allAccounts, detected)
	if matched == nil {
		t.Fatal("expected to match stored account by PUUID")
	}
	if matched.ID != stored.ID {
		t.Errorf("matched wrong account: got %q, want %q", matched.ID, stored.ID)
	}

	// Update + persist
	app.Providers.UpdateAccount(matched, detected)
	if _, err := app.Accounts.Update(*matched); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify persisted ranks
	reread, err := app.Accounts.GetByID(stored.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(reread.CachedRanks) != 1 {
		t.Fatalf("expected 1 cached rank, got %d", len(reread.CachedRanks))
	}
	if reread.CachedRanks[0].Tier != "DIAMOND" {
		t.Errorf("rank not persisted: %#v", reread.CachedRanks[0])
	}
}

// TestDetection_NoMatchForUnknownAccount verifies that when a detected
// account doesn't match any stored account, MatchAccount returns nil
// (and the UI would prompt to link).
func TestDetection_NoMatchForUnknownAccount(t *testing.T) {
	app := newTestApp(t)
	if err := app.Storage.CreateVault("user", "pw"); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}

	// Store an account with a DIFFERENT PUUID than what we'll detect
	if _, err := app.Accounts.Create(models.Account{
		Username:  "stored",
		NetworkID: "riot",
		PUUID:     "stored-puuid",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	app.RiotFake.Detected = &providers.DetectedAccount{
		NetworkID: "riot",
		UniqueID:  "different-puuid",
		PUUID:     "different-puuid",
		RiotID:    "Stranger#EU",
	}

	detected, err := app.Providers.DetectAny(context.Background())
	if err != nil {
		t.Fatalf("DetectAny: %v", err)
	}

	allAccounts, _ := app.Accounts.GetAll()
	matched := app.Providers.MatchAccount(allAccounts, detected)
	if matched != nil {
		t.Errorf("expected no match for unknown PUUID, got %#v", matched)
	}
}

// TestDetection_IsAnyClientRunning_TogglesWithProviderState verifies the
// "is any client up" check that drives polling decisions.
func TestDetection_IsAnyClientRunning_TogglesWithProviderState(t *testing.T) {
	app := newTestApp(t)

	if app.Providers.IsAnyClientRunning(context.Background()) {
		t.Fatal("no clients should be running initially")
	}

	app.RiotFake.ClientRunning = true
	if !app.Providers.IsAnyClientRunning(context.Background()) {
		t.Fatal("registry should report a client running when fake says so")
	}
}
