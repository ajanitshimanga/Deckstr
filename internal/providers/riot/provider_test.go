package riot_test

import (
	"testing"

	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/contract"
	"OpenSmurfManager/internal/providers/riot"
)

// TestRiotProvider_Contract runs the standard provider conformance suite
// against the Riot adapter. Live Detect tests are skipped because they
// require a running League client - the offline behavior is verified
// via the Detect_ReturnsDetectionErrorWhenOffline subtest which the
// non-skip path exercises.
func TestRiotProvider_Contract(t *testing.T) {
	contract.RunContractTests(t, contract.Suite{
		Factory: func() providers.Provider { return riot.New() },
	})
}
