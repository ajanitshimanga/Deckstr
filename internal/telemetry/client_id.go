package telemetry

import (
	"os"
	"strings"

	"github.com/google/uuid"
)

// loadOrCreateClientID returns a stable anonymous identifier for this
// install. The value is generated once, written to disk, and reused on
// every subsequent launch — giving backends a way to compute DAU/MAU
// without ever seeing any user data.
//
// path is the absolute file path (caller-supplied so it can be overridden
// in tests).
func loadOrCreateClientID(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if _, perr := uuid.Parse(id); perr == nil {
			return id, nil
		}
		// Fall through to regenerate on a corrupted file.
	}

	id := uuid.NewString()
	if err := os.WriteFile(path, []byte(id+"\n"), 0600); err != nil {
		return "", err
	}
	return id, nil
}
