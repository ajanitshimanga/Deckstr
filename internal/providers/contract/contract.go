// Package contract provides a reusable test suite that validates any
// providers.Provider implementation against the interface contract.
//
// New platform providers (Epic, Steam, Battle.net, etc.) should add a test
// in their own package that calls RunContractTests with a factory. Passing
// the contract is a prerequisite for accepting the provider into the registry.
//
// Example:
//
//	func TestMyProvider_Contract(t *testing.T) {
//	    contract.RunContractTests(t, contract.Suite{
//	        Factory:        func() providers.Provider { return myprovider.New() },
//	        SkipLiveDetect: true, // detect requires a real client
//	    })
//	}
package contract

import (
	"context"
	"errors"
	"testing"
	"time"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
)

// Suite configures a contract test run.
type Suite struct {
	// Factory creates a fresh provider instance for each test. Required.
	Factory func() providers.Provider

	// SkipLiveDetect skips Detect() tests that require real client behavior.
	// Use this for providers that talk to external processes - the contract
	// only verifies graceful behavior when offline, which all providers must
	// support.
	SkipLiveDetect bool

	// DetectTimeout bounds how long a Detect call may take when the client
	// is offline. Defaults to 5s if zero.
	DetectTimeout time.Duration
}

// RunContractTests executes the full provider conformance suite against
// the given factory. Each subtest creates a fresh provider instance.
func RunContractTests(t *testing.T, suite Suite) {
	t.Helper()

	if suite.Factory == nil {
		t.Fatal("contract.Suite.Factory is required")
	}
	if suite.DetectTimeout == 0 {
		suite.DetectTimeout = 5 * time.Second
	}

	t.Run("NetworkID_NonEmptyAndStable", func(t *testing.T) {
		p := suite.Factory()
		id := p.NetworkID()
		if id == "" {
			t.Fatal("NetworkID() must return a non-empty string")
		}
		if again := p.NetworkID(); again != id {
			t.Fatalf("NetworkID() must be stable: got %q then %q", id, again)
		}
	})

	t.Run("DisplayName_NonEmpty", func(t *testing.T) {
		p := suite.Factory()
		if p.DisplayName() == "" {
			t.Fatal("DisplayName() must return a non-empty string for UI use")
		}
	})

	t.Run("IsClientRunning_SafeToCallWhenOffline", func(t *testing.T) {
		// Should never panic, regardless of whether a client is present.
		p := suite.Factory()
		ctx, cancel := context.WithTimeout(context.Background(), suite.DetectTimeout)
		defer cancel()
		_ = p.IsClientRunning(ctx)
	})

	t.Run("IsClientRunning_RespectsCancelledContext", func(t *testing.T) {
		p := suite.Factory()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Should return promptly, not hang. We can't assert false because
		// some providers don't use ctx for the cheap process check.
		done := make(chan struct{})
		go func() {
			_ = p.IsClientRunning(ctx)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(suite.DetectTimeout):
			t.Fatal("IsClientRunning did not return promptly with cancelled context")
		}
	})

	if !suite.SkipLiveDetect {
		t.Run("Detect_ReturnsDetectionErrorWhenOffline", func(t *testing.T) {
			p := suite.Factory()
			ctx, cancel := context.WithTimeout(context.Background(), suite.DetectTimeout)
			defer cancel()

			detected, err := p.Detect(ctx)

			// If a client happens to be running during the test, skip rather
			// than fail - we can't control the host environment.
			if err == nil && detected != nil {
				t.Skip("a real client is running on the test host; skipping offline assertion")
			}

			if err == nil {
				t.Fatal("Detect must return either a result or an error, not (nil, nil)")
			}

			var detErr *providers.DetectionError
			if !errors.As(err, &detErr) {
				t.Fatalf("offline detection must return *providers.DetectionError so callers can distinguish expected states; got %T: %v", err, err)
			}

			if detErr.Code == "" {
				t.Error("DetectionError.Code must be set so callers can branch on stable codes")
			}
			if detErr.Message == "" {
				t.Error("DetectionError.Message must be human-readable")
			}
		})
	}

	t.Run("Detect_PopulatesNetworkIDOnSuccess", func(t *testing.T) {
		// We can't force success without a real client, but if Detect does
		// happen to succeed (e.g. live test environment), the contract still
		// applies: NetworkID must match.
		p := suite.Factory()
		ctx, cancel := context.WithTimeout(context.Background(), suite.DetectTimeout)
		defer cancel()

		detected, err := p.Detect(ctx)
		if err != nil || detected == nil {
			t.Skip("no live session - cannot verify success-case fields")
		}

		if detected.NetworkID != p.NetworkID() {
			t.Errorf("DetectedAccount.NetworkID = %q, must match Provider.NetworkID() = %q",
				detected.NetworkID, p.NetworkID())
		}
		if detected.DisplayName == "" {
			t.Error("DetectedAccount.DisplayName must be set on success for UI display")
		}
		if detected.UniqueID == "" {
			t.Error("DetectedAccount.UniqueID must be set on success for matching")
		}
		if detected.DetectedAt.IsZero() {
			t.Error("DetectedAccount.DetectedAt must be set on success")
		}
	})

	t.Run("MatchAccount_ReturnsNilForEmptyList", func(t *testing.T) {
		p := suite.Factory()
		got := p.MatchAccount(nil, &providers.DetectedAccount{
			NetworkID: p.NetworkID(),
			UniqueID:  "anything",
		})
		if got != nil {
			t.Fatalf("MatchAccount with no accounts must return nil, got %#v", got)
		}
	})

	t.Run("MatchAccount_ReturnsNilForNilDetected", func(t *testing.T) {
		p := suite.Factory()
		accounts := []models.Account{{ID: "a", NetworkID: p.NetworkID()}}
		got := p.MatchAccount(accounts, nil)
		if got != nil {
			t.Fatalf("MatchAccount with nil detected must return nil, got %#v", got)
		}
	})

	t.Run("MatchAccount_DoesNotMatchOtherNetworks", func(t *testing.T) {
		p := suite.Factory()
		// Account from a different network must never match, even if
		// other identifiers happen to overlap.
		accounts := []models.Account{
			{ID: "other-network-acc", NetworkID: "definitely-not-" + p.NetworkID()},
		}
		got := p.MatchAccount(accounts, &providers.DetectedAccount{
			NetworkID: p.NetworkID(),
			UniqueID:  "shared-id",
		})
		if got != nil {
			t.Fatalf("MatchAccount must not return an account from a different network: %#v", got)
		}
	})

	t.Run("UpdateAccount_NilSafe", func(t *testing.T) {
		p := suite.Factory()
		// Calling with nil args must not panic.
		p.UpdateAccount(nil, nil)
		p.UpdateAccount(nil, &providers.DetectedAccount{NetworkID: p.NetworkID()})
		p.UpdateAccount(&models.Account{NetworkID: p.NetworkID()}, nil)
	})

	t.Run("UpdateAccount_PreservesUnrelatedFields", func(t *testing.T) {
		p := suite.Factory()
		account := &models.Account{
			ID:        "preserved-id",
			NetworkID: p.NetworkID(),
			Username:  "preserved-username",
			Notes:     "preserved-notes",
		}
		detected := &providers.DetectedAccount{
			NetworkID:   p.NetworkID(),
			DisplayName: "detected-name",
			UniqueID:    "detected-unique",
			DetectedAt:  time.Now(),
		}

		p.UpdateAccount(account, detected)

		if account.ID != "preserved-id" {
			t.Errorf("UpdateAccount must not change Account.ID, got %q", account.ID)
		}
		if account.Username != "preserved-username" {
			t.Errorf("UpdateAccount must not change Account.Username, got %q", account.Username)
		}
		if account.Notes != "preserved-notes" {
			t.Errorf("UpdateAccount must not change Account.Notes, got %q", account.Notes)
		}
	})
}
