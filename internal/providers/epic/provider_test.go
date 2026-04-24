package epic_test

import (
	"context"
	"errors"
	"testing"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/contract"
	"OpenSmurfManager/internal/providers/epic"
)

// TestEpicProvider_Contract runs the standard provider conformance suite
// against the Epic adapter. Live detection is not meaningful here (Detect
// always returns a structured DetectionError today), but the contract's
// offline path still applies.
func TestEpicProvider_Contract(t *testing.T) {
	contract.RunContractTests(t, contract.Suite{
		Factory: func() providers.Provider { return epic.New() },
	})
}

// TestEpicProvider_NetworkIDMatchesModel ensures the provider's NetworkID
// lines up with the entry registered in models.DefaultGameNetworks — a
// mismatch would silently hide Epic accounts from the polling loop.
func TestEpicProvider_NetworkIDMatchesModel(t *testing.T) {
	p := epic.New()
	for _, n := range models.DefaultGameNetworks() {
		if n.ID == p.NetworkID() {
			return
		}
	}
	t.Fatalf("epic provider NetworkID %q not present in DefaultGameNetworks", p.NetworkID())
}

// TestEpicProvider_DetectReportsNotSignedInOrOffline verifies that Detect
// always returns a structured DetectionError — account auto-detection is
// intentionally unimplemented today, and the registry relies on the stable
// error codes to move on to the next provider.
func TestEpicProvider_DetectReportsNotSignedInOrOffline(t *testing.T) {
	p := epic.New()
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

	// Either code is acceptable depending on whether the launcher is running
	// on the test host. Both are structured signals, which is the contract.
	switch detErr.Code {
	case providers.ErrCodeClientOffline, providers.ErrCodeNotSignedIn:
	default:
		t.Fatalf("unexpected DetectionError.Code %q", detErr.Code)
	}
}
