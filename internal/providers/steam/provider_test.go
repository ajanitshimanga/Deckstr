package steam_test

import (
	"context"
	"errors"
	"testing"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/contract"
	"OpenSmurfManager/internal/providers/steam"
)

// TestSteamProvider_Contract runs the standard provider conformance suite
// against the Steam adapter.
func TestSteamProvider_Contract(t *testing.T) {
	contract.RunContractTests(t, contract.Suite{
		Factory: func() providers.Provider { return steam.New() },
	})
}

// TestSteamProvider_NetworkIDMatchesModel ensures the provider's NetworkID
// matches the entry registered in models.DefaultGameNetworks.
func TestSteamProvider_NetworkIDMatchesModel(t *testing.T) {
	p := steam.New()
	for _, n := range models.DefaultGameNetworks() {
		if n.ID == p.NetworkID() {
			return
		}
	}
	t.Fatalf("steam provider NetworkID %q not present in DefaultGameNetworks", p.NetworkID())
}

// TestSteamProvider_DetectReportsNotSignedInOrOffline verifies that Detect
// always returns a structured DetectionError — Steam auto-detection is not
// implemented yet, and the registry relies on the stable error codes.
func TestSteamProvider_DetectReportsNotSignedInOrOffline(t *testing.T) {
	p := steam.New()
	detected, err := p.Detect(context.Background())

	if detected != nil {
		t.Fatalf("Detect must not return a detected account yet, got %#v", detected)
	}
	if err == nil {
		t.Fatal("Detect must return a DetectionError, got nil")
	}

	var detErr *providers.DetectionError
	if !errors.As(err, &detErr) {
		t.Fatalf("Detect must return *providers.DetectionError, got %T: %v", err, err)
	}

	switch detErr.Code {
	case providers.ErrCodeClientOffline, providers.ErrCodeNotSignedIn:
	default:
		t.Fatalf("unexpected DetectionError.Code %q", detErr.Code)
	}
}
