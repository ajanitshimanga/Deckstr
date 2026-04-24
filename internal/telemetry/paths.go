package telemetry

import (
	"fmt"
	"os"
	"path/filepath"

	"OpenSmurfManager/internal/appdir"
)

const (
	logsDirName       = "logs"
	logFileName       = "app.log"
	clientIDFile      = "client.id"
	defaultMaxSize    = 1 * 1024 * 1024 // 1 MB per file
	defaultBackups    = 3               // keep .1, .2, .3 → 4 files total (~4 MB cap)
	defaultFlushEvery = 5               // seconds
)

// logsDir returns the directory where rotated log files live, sitting under
// the shared per-user app dir (handled by internal/appdir, including the
// rebrand migration from the legacy folder name).
func logsDir() (string, error) {
	app, err := appdir.Path()
	if err != nil {
		return "", fmt.Errorf("telemetry: app dir: %w", err)
	}
	dir := filepath.Join(app, logsDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("telemetry: mkdir logs: %w", err)
	}
	return dir, nil
}

// clientIDPath returns the path to the anonymous client ID file.
// Stored next to the vault, outside the encrypted blob so it's available
// before unlock (needed to emit pre-unlock events like app.start).
func clientIDPath() (string, error) {
	app, err := appdir.Path()
	if err != nil {
		return "", fmt.Errorf("telemetry: app dir: %w", err)
	}
	return filepath.Join(app, clientIDFile), nil
}
