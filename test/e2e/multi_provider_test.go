package e2e

import (
	"context"
	"testing"
	"time"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/fake"
)

// TestMultiProvider_DispatchByNetworkID is the canonical proof that
// multiple platforms can coexist in the registry without interfering.
// When a provider for a new platform (e.g. Epic) is added, this test
// pattern should be extended to cover it.
func TestMultiProvider_DispatchByNetworkID(t *testing.T) {
	app := newTestApp(t)
	if err := app.Storage.CreateVault("user", "pw"); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}

	// Add a second provider for a hypothetical "epic" platform. We give it
	// a MatchFunc that simulates platform-specific matching (Epic might use
	// email, Steam might use SteamID, etc.).
	epic := fake.New("epic", "Epic Games (Fake)")
	epic.MatchFunc = func(accounts []models.Account, detected *providers.DetectedAccount) *models.Account {
		for i := range accounts {
			if accounts[i].NetworkID == "epic" && accounts[i].DisplayName == detected.DisplayName {
				return &accounts[i]
			}
		}
		return nil
	}
	app.Providers.MustRegister(epic)

	// Store accounts for BOTH platforms using only generic, platform-neutral fields.
	riotAcc, err := app.Accounts.Create(models.Account{
		Username: "riot-acc", NetworkID: "riot", DisplayName: "RiotPlayer",
	})
	if err != nil {
		t.Fatalf("Create riot: %v", err)
	}
	epicAcc, err := app.Accounts.Create(models.Account{
		Username: "epic-acc", NetworkID: "epic", DisplayName: "EpicPlayer",
	})
	if err != nil {
		t.Fatalf("Create epic: %v", err)
	}

	// Riot offline, Epic online with a matching detection
	app.RiotFake.DetectErr = &providers.DetectionError{
		Code: providers.ErrCodeClientOffline, Message: "no riot client",
	}
	epic.Detected = &providers.DetectedAccount{
		NetworkID:   "epic",
		DisplayName: "EpicPlayer",
		UniqueID:    "epic-unique-id",
		DetectedAt:  time.Now(),
	}

	// Detection should skip Riot (offline) and return Epic
	detected, err := app.Providers.DetectAny(context.Background())
	if err != nil {
		t.Fatalf("DetectAny: %v", err)
	}
	if detected == nil {
		t.Fatal("expected to detect Epic account when Riot offline")
	}
	if detected.NetworkID != "epic" {
		t.Errorf("expected NetworkID=epic, got %q", detected.NetworkID)
	}

	// Match should dispatch to the Epic provider, not Riot
	all, _ := app.Accounts.GetAll()
	matched := app.Providers.MatchAccount(all, detected)
	if matched == nil {
		t.Fatal("expected to match Epic account")
	}
	if matched.ID != epicAcc.ID {
		t.Errorf("matched wrong account: got %q, want %q (Epic)", matched.ID, epicAcc.ID)
	}

	// Verify the Riot fake was NEVER asked to match (dispatch isolation)
	if app.RiotFake.Stats().MatchCalls != 0 {
		t.Errorf("riot provider should not receive match calls for epic detection; got %d",
			app.RiotFake.Stats().MatchCalls)
	}
	if epic.Stats().MatchCalls == 0 {
		t.Error("epic provider should have received the match call")
	}

	// Confirm the Riot account was untouched
	stillRiot, err := app.Accounts.GetByID(riotAcc.ID)
	if err != nil {
		t.Fatalf("GetByID riot: %v", err)
	}
	if stillRiot.NetworkID != "riot" {
		t.Errorf("riot account network changed: %q", stillRiot.NetworkID)
	}
}

// TestMultiProvider_AllOfflineReturnsNil verifies that when ALL providers
// report offline, DetectAny returns (nil, nil) - the polling loop should
// treat this as "nothing to do, try again later".
func TestMultiProvider_AllOfflineReturnsNil(t *testing.T) {
	app := newTestApp(t)
	epic := fake.New("epic", "Epic Games")
	app.Providers.MustRegister(epic)

	offlineErr := &providers.DetectionError{
		Code: providers.ErrCodeClientOffline, Message: "offline",
	}
	app.RiotFake.DetectErr = offlineErr
	epic.DetectErr = offlineErr

	got, err := app.Providers.DetectAny(context.Background())
	if err != nil {
		t.Errorf("expected no error when all providers cleanly offline, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil detection, got %#v", got)
	}
}
