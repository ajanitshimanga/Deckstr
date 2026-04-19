package contract_test

import (
	"testing"

	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/contract"
	"OpenSmurfManager/internal/providers/fake"
)

// TestContract_FakeProviderOffline verifies the contract suite passes when
// run against a properly-implemented fake that simulates an offline client.
// This is the canonical "what a passing contract looks like" test.
func TestContract_FakeProviderOffline(t *testing.T) {
	contract.RunContractTests(t, contract.Suite{
		Factory: func() providers.Provider {
			p := fake.New("test", "Test Platform")
			p.DetectErr = &providers.DetectionError{
				Code:    providers.ErrCodeClientOffline,
				Message: "fake client is not running",
				Retry:   true,
			}
			return p
		},
	})
}
