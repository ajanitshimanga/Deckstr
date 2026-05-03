package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveIn_FreshInstall pins that a brand-new install (no legacy dir,
// no current dir) creates the current dir and returns its path.
func TestResolveIn_FreshInstall(t *testing.T) {
	root := t.TempDir()

	got, err := resolveIn(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(root, CurrentName)
	if got != want {
		t.Errorf("resolveIn = %q, want %q", got, want)
	}
	if !dirExistsT(t, want) {
		t.Errorf("expected %q to exist on disk after resolveIn", want)
	}
}

// TestResolveIn_MigratesFromLegacy pins the rebrand-day behaviour: an
// existing OpenSmurfManager dir with a vault inside is renamed to Deckstr,
// preserving its contents.
func TestResolveIn_MigratesFromLegacy(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyName)
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatalf("seed legacy dir: %v", err)
	}
	// Drop a sentinel file so we can verify it survives the rename.
	sentinelPath := filepath.Join(legacy, "vault.osm")
	sentinelData := []byte("legacy-vault-bytes")
	if err := os.WriteFile(sentinelPath, sentinelData, 0600); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}

	got, err := resolveIn(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(root, CurrentName)
	if got != want {
		t.Errorf("resolveIn = %q, want %q", got, want)
	}
	// The legacy dir must be gone — single rename, not a copy.
	if dirExistsT(t, legacy) {
		t.Errorf("legacy dir %q should not exist after migration", legacy)
	}
	// Sentinel must be readable at the new location with original bytes.
	migratedSentinel := filepath.Join(want, "vault.osm")
	gotBytes, err := os.ReadFile(migratedSentinel)
	if err != nil {
		t.Fatalf("read migrated sentinel: %v", err)
	}
	if string(gotBytes) != string(sentinelData) {
		t.Errorf("migrated sentinel bytes = %q, want %q", gotBytes, sentinelData)
	}
}

// TestResolveIn_PrefersCurrentWhenBothExist pins that if a user happens to
// have both directories on disk (manual recovery, partial migration that
// resumed), the current dir wins and the legacy one is left untouched for
// the user to inspect/delete manually.
func TestResolveIn_PrefersCurrentWhenBothExist(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyName)
	current := filepath.Join(root, CurrentName)
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatalf("seed current: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "vault.osm"), []byte("legacy"), 0600); err != nil {
		t.Fatalf("seed legacy file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(current, "vault.osm"), []byte("current"), 0600); err != nil {
		t.Fatalf("seed current file: %v", err)
	}

	got, err := resolveIn(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != current {
		t.Errorf("resolveIn = %q, want %q (current must win)", got, current)
	}
	if !dirExistsT(t, legacy) {
		t.Errorf("legacy dir should remain when current already exists")
	}
}

// TestResolveIn_MergesSplitState pins the recovery path for partially-failed
// migrations: legacy still has a vault.osm but current already exists with
// some files (say, freshly-created logs). The merge moves the legacy vault
// into current without overwriting anything, then drops the legacy dir.
func TestResolveIn_MergesSplitState(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyName)
	current := filepath.Join(root, CurrentName)
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatalf("seed current: %v", err)
	}
	// Legacy has the user's real vault. Current has fresh telemetry artifacts
	// (mimicking the broken state where another process created current first).
	realVault := []byte("real-vault-data")
	if err := os.WriteFile(filepath.Join(legacy, "vault.osm"), realVault, 0600); err != nil {
		t.Fatalf("seed legacy vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(current, "client.id"), []byte("abc-123"), 0600); err != nil {
		t.Fatalf("seed current client.id: %v", err)
	}

	got, err := resolveIn(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != current {
		t.Errorf("resolveIn = %q, want %q", got, current)
	}
	// Legacy vault must have moved into current.
	migrated, err := os.ReadFile(filepath.Join(current, "vault.osm"))
	if err != nil {
		t.Fatalf("read migrated vault: %v", err)
	}
	if string(migrated) != string(realVault) {
		t.Errorf("migrated vault bytes = %q, want %q", migrated, realVault)
	}
	// Legacy dir should be gone since merge cleared it.
	if dirExistsT(t, legacy) {
		t.Errorf("legacy dir should be removed after a clean merge")
	}
}

// TestResolveIn_MergeNeverOverwrites pins the safety property: if both dirs
// hold a vault.osm, the current one wins and the legacy file stays in place
// for manual recovery. Silent overwrite of vault data is the worst possible
// failure mode for a password manager.
func TestResolveIn_MergeNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyName)
	current := filepath.Join(root, CurrentName)
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatalf("seed current: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "vault.osm"), []byte("legacy-real"), 0600); err != nil {
		t.Fatalf("seed legacy vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(current, "vault.osm"), []byte("current-stub"), 0600); err != nil {
		t.Fatalf("seed current vault: %v", err)
	}

	if _, err := resolveIn(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	currentBytes, _ := os.ReadFile(filepath.Join(current, "vault.osm"))
	if string(currentBytes) != "current-stub" {
		t.Errorf("current vault was overwritten: got %q, want %q", currentBytes, "current-stub")
	}
	legacyBytes, err := os.ReadFile(filepath.Join(legacy, "vault.osm"))
	if err != nil {
		t.Fatalf("legacy vault was destroyed: %v", err)
	}
	if string(legacyBytes) != "legacy-real" {
		t.Errorf("legacy vault was modified: got %q, want %q", legacyBytes, "legacy-real")
	}
}

// TestLegacyVaultPath_NoLegacyDir pins the side-effect-free probe contract:
// when there's no OpenSmurfManager folder under the user's config dir, the
// helper returns ("", nil) instead of creating anything or erroring.
func TestLegacyVaultPath_NoLegacyDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)

	got, err := LegacyVaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("LegacyVaultPath = %q, want empty", got)
	}
}

// TestLegacyVaultPath_FindsOrphanedFile pins that the helper surfaces the
// exact path of an orphaned vault.osm sitting in a stale OpenSmurfManager
// folder, so the storage layer can read its header without re-running the
// full Path() migration (which has side effects).
func TestLegacyVaultPath_FindsOrphanedFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)

	legacyDir := filepath.Join(root, LegacyName)
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatalf("seed legacy dir: %v", err)
	}
	legacyVault := filepath.Join(legacyDir, "vault.osm")
	if err := os.WriteFile(legacyVault, []byte("legacy"), 0600); err != nil {
		t.Fatalf("seed legacy vault: %v", err)
	}

	got, err := LegacyVaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != legacyVault {
		t.Errorf("LegacyVaultPath = %q, want %q", got, legacyVault)
	}
	// And the call must not have moved or deleted anything.
	if _, err := os.Stat(legacyVault); err != nil {
		t.Errorf("legacy vault disappeared: %v", err)
	}
}

func dirExistsT(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("stat %q: %v", path, err)
	}
	return info.IsDir()
}
