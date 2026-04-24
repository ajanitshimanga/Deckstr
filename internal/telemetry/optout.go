package telemetry

import (
	"os"
	"path/filepath"

	"OpenSmurfManager/internal/appdir"
)

// optOutFileName is a sentinel dropped next to the vault when the user
// unticks the telemetry checkbox in the installer (or toggles the setting
// off at runtime). Presence = disabled; absence = enabled.
const optOutFileName = "telemetry.disabled"

// IsDisabled reports whether the user has opted out of telemetry. On any
// error resolving the app dir we fail safe (treat as disabled) so a
// misconfigured host never silently enables logging.
func IsDisabled() bool {
	path, err := optOutPath()
	if err != nil {
		return true
	}
	return isDisabledAt(path)
}

// SetDisabled writes or removes the opt-out marker so the next launch
// reflects the new choice. Exposed for a future Settings → Usage
// Analytics toggle.
func SetDisabled(disabled bool) error {
	path, err := optOutPath()
	if err != nil {
		return err
	}
	return setDisabledAt(path, disabled)
}

// isDisabledAt is the path-parameterised core of IsDisabled — the split
// lets tests drive the marker-file state against a tmpdir without
// touching the real user config directory.
func isDisabledAt(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// setDisabledAt is the path-parameterised core of SetDisabled. Idempotent
// in both directions: writing a marker that already exists is a no-op,
// and removing one that doesn't exist is silent.
func setDisabledAt(path string, disabled bool) error {
	if disabled {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		return f.Close()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func optOutPath() (string, error) {
	app, err := appdir.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(app, optOutFileName), nil
}

// LogsPath returns the directory on disk where rotated usage logs live.
// Surfaced to the UI so the Settings panel can open it in the user's file
// manager for transparency. Creates the directory if it doesn't yet exist
// (e.g., user is opening the folder before any event has been logged).
func LogsPath() (string, error) {
	return logsDir()
}
