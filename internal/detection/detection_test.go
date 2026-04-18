package detection

import (
	"testing"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/riotclient"
)

func TestMatchStoredAccountRiotByPUUID(t *testing.T) {
	accounts := []models.Account{
		{ID: "1", NetworkID: "riot", RiotID: "first#NA1", PUUID: "puuid-1"},
		{ID: "2", NetworkID: "epic", Username: "player@example.com"},
	}

	detected := &riotclient.DetectedAccount{
		NetworkID: "riot",
		RiotID:    "other#TAG",
		PUUID:     "puuid-1",
	}

	matched := MatchStoredAccount(accounts, detected)
	if matched == nil || matched.ID != "1" {
		t.Fatalf("expected Riot account 1 to match by PUUID, got %#v", matched)
	}
}

func TestMatchStoredAccountEpicUsesLinkedEmailAndUsernameFallback(t *testing.T) {
	accounts := []models.Account{
		{ID: "1", NetworkID: "epic", Username: "alt@example.com"},
		{ID: "2", NetworkID: "epic", EpicEmail: "linked@example.com", Username: "different@example.com"},
	}

	matched := MatchStoredAccount(accounts, &riotclient.DetectedAccount{
		NetworkID: "epic",
		Email:     "linked@example.com",
	})
	if matched == nil || matched.ID != "2" {
		t.Fatalf("expected Epic account 2 to match by linked email, got %#v", matched)
	}

	matched = MatchStoredAccount(accounts, &riotclient.DetectedAccount{
		NetworkID: "epic",
		Email:     "alt@example.com",
	})
	if matched == nil || matched.ID != "1" {
		t.Fatalf("expected Epic account 1 to match by username fallback, got %#v", matched)
	}
}

func TestApplyDetectionEpicCachesEmailAndDisplayName(t *testing.T) {
	account := &models.Account{NetworkID: "epic"}
	detected := &riotclient.DetectedAccount{
		NetworkID:   "epic",
		DisplayName: "SquishyPig",
		Email:       "player@example.com",
	}

	ApplyDetection(account, detected)

	if account.EpicEmail != "player@example.com" {
		t.Fatalf("expected Epic email to be cached, got %q", account.EpicEmail)
	}
	if account.DisplayName != "SquishyPig" {
		t.Fatalf("expected display name to be filled from detection, got %q", account.DisplayName)
	}
}
