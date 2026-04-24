package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	appDirName      = "OpenSmurfManager"
	logsDirName     = "logs"
	logFileName     = "app.log"
	clientIDFile    = "client.id"
	defaultMaxSize  = 1 * 1024 * 1024 // 1 MB per file
	defaultBackups  = 3               // keep .1, .2, .3 → 4 files total (~4 MB cap)
	defaultFlushEvery = 5             // seconds
)

// logsDir returns the directory where rotated log files live.
// Matches the vault location convention (os.UserConfigDir/OpenSmurfManager).
func logsDir() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("telemetry: user config dir: %w", err)
	}
	dir := filepath.Join(config, appDirName, logsDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("telemetry: mkdir logs: %w", err)
	}
	return dir, nil
}

// clientIDPath returns the path to the anonymous client ID file.
// Stored next to the vault, outside the encrypted blob so it's available
// before unlock (needed to emit pre-unlock events like app.start).
func clientIDPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("telemetry: user config dir: %w", err)
	}
	dir := filepath.Join(config, appDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("telemetry: mkdir app: %w", err)
	}
	return filepath.Join(dir, clientIDFile), nil
}
