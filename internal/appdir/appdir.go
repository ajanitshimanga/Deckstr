// Package appdir resolves the user-config directory the app stores its data
// under and handles the one-shot migration from the legacy "OpenSmurfManager"
// folder to the current "Deckstr" name.
//
// Both storage and telemetry call Path() — the migration runs at most once
// per process and is idempotent across calls, so call ordering between
// packages doesn't matter.
package appdir

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// CurrentName is the on-disk directory the app writes to today.
	CurrentName = "Deckstr"
	// LegacyName is the pre-rebrand directory we migrate from on first boot.
	// Kept exported so tests + tooling can reason about the migration window.
	LegacyName = "OpenSmurfManager"
)

var (
	once   sync.Once
	cached string
	cerr   error
)

// Path returns the absolute path to the app's per-user data directory,
// creating it (and migrating from the legacy name when applicable) on the
// first call. Subsequent calls return the cached value.
func Path() (string, error) {
	once.Do(func() {
		config, err := os.UserConfigDir()
		if err != nil {
			cerr = fmt.Errorf("appdir: user config dir: %w", err)
			return
		}
		cached, cerr = resolveIn(config)
	})
	return cached, cerr
}

// resolveIn picks the active directory under the given config root, handling
// the legacy → current migration in three modes:
//
//  1. Fresh install: create current, return.
//  2. Pure legacy (legacy exists, current doesn't): atomic os.Rename — fastest
//     and survives a crash mid-migration since rename is journaled.
//  3. Split state (both dirs exist): walk legacy and move every entry that
//     isn't already in current. Files in current always win — we never
//     overwrite, so a corrupted half-migration that left an empty current
//     dir doesn't destroy the legacy data. Once the merge is done, remove
//     legacy if it's empty.
//
// Exported via the unexported name for tests that drive resolution against
// a controlled tmpdir without touching real os.UserConfigDir state.
func resolveIn(config string) (string, error) {
	current := filepath.Join(config, CurrentName)
	legacy := filepath.Join(config, LegacyName)

	currentExists, err := dirExists(current)
	if err != nil {
		return "", err
	}
	legacyExists, err := dirExists(legacy)
	if err != nil {
		return "", err
	}

	switch {
	case !currentExists && legacyExists:
		// Mode 2: atomic rename. If it fails (permission, file handle), fall
		// through to merge mode below by ensuring current exists.
		if err := os.Rename(legacy, current); err == nil {
			return current, nil
		}
		// Rename failed — create current and fall into merge.
		if err := os.MkdirAll(current, 0700); err != nil {
			return "", fmt.Errorf("appdir: mkdir after rename failed: %w", err)
		}
		fallthrough
	case currentExists && legacyExists:
		// Mode 3: per-entry merge. Don't overwrite anything in current — if
		// both dirs hold a vault.osm, current wins (likely freshly created;
		// the legacy one stays in place for manual recovery rather than
		// being silently clobbered).
		if err := mergeLegacyIntoCurrent(legacy, current); err != nil {
			return "", err
		}
		return current, nil
	default:
		// Mode 1: fresh install (or current already present, no legacy).
		if err := os.MkdirAll(current, 0700); err != nil {
			return "", fmt.Errorf("appdir: mkdir: %w", err)
		}
		return current, nil
	}
}

// mergeLegacyIntoCurrent moves every entry from legacy into current that
// doesn't already exist in current. Conservative by design: never overwrites,
// never deletes data it can't move. Removes legacy only if empty afterwards.
func mergeLegacyIntoCurrent(legacy, current string) error {
	entries, err := os.ReadDir(legacy)
	if err != nil {
		return fmt.Errorf("appdir: read legacy: %w", err)
	}
	for _, entry := range entries {
		src := filepath.Join(legacy, entry.Name())
		dst := filepath.Join(current, entry.Name())
		if _, err := os.Stat(dst); err == nil {
			// Already exists in current — skip to avoid overwriting.
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("appdir: stat %s: %w", dst, err)
		}
		if err := os.Rename(src, dst); err != nil {
			// Single-entry rename failure is non-fatal: leave it in legacy
			// for manual recovery rather than aborting the whole merge.
			continue
		}
	}
	// Remove legacy only if completely empty post-merge.
	remaining, err := os.ReadDir(legacy)
	if err == nil && len(remaining) == 0 {
		_ = os.Remove(legacy)
	}
	return nil
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("appdir: stat %s: %w", path, err)
}

// LegacyVaultPath returns the absolute path of a vault.osm sitting in the
// pre-rebrand OpenSmurfManager directory, or "" if no such file is on disk.
// Pure probe — does NOT trigger Path()'s migration side effects.
//
// Used by the storage layer to surface an "adopt my old vault" recovery
// option when Path()'s "current wins" merge silently left a real vault
// behind (the bug a small number of v1.5→1.6 upgraders hit).
func LegacyVaultPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("appdir: user config dir: %w", err)
	}
	candidate := filepath.Join(config, LegacyName, "vault.osm")
	if _, err := os.Stat(candidate); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("appdir: stat %s: %w", candidate, err)
	}
	return candidate, nil
}

