package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenSmurfManager/internal/crypto"
	"OpenSmurfManager/internal/models"
)

// newServiceWithLegacy returns a storage service whose current vault path
// and legacy override both point under tmpDir. Tests can populate either or
// both files to cover the migration scenarios.
func newServiceWithLegacy(t *testing.T) (svc *StorageService, currentPath, legacyPath string, cleanup func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "osm-legacy-*")
	if err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	currentPath = filepath.Join(tmpDir, "current", "vault.osm")
	legacyPath = filepath.Join(tmpDir, "legacy", "vault.osm")
	if err := os.MkdirAll(filepath.Dir(currentPath), 0700); err != nil {
		t.Fatalf("mkdir current: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	svc = NewStorageServiceWithPath(currentPath)
	svc.legacyVaultPathOverride = legacyPath
	return svc, currentPath, legacyPath, func() { os.RemoveAll(tmpDir) }
}

// writeFakeVault writes a minimally-valid Vault JSON to path so detection has
// real metadata to parse. We use a fixed-size random salt/nonce so the file
// shape matches a real vault — DetectLegacyVault doesn't decrypt, but the
// JSON unmarshal must succeed.
func writeFakeVault(t *testing.T, path, username string, version int) {
	t.Helper()
	v := models.Vault{
		Version:       version,
		Username:      username,
		Salt:          crypto.EncodeBase64([]byte("aaaaaaaaaaaaaaaa")),
		Nonce:         crypto.EncodeBase64([]byte("bbbbbbbbbbbb")),
		EncryptedData: crypto.EncodeBase64([]byte("ciphertext-bytes")),
		CreatedAt:     time.Unix(1700000000, 0),
		UpdatedAt:     time.Unix(1700000001, 0),
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal vault: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write fake vault: %v", err)
	}
}

func TestDetectLegacyVault_NoLegacy(t *testing.T) {
	svc, _, _, cleanup := newServiceWithLegacy(t)
	defer cleanup()

	got, err := svc.DetectLegacyVault()
	if err != nil {
		t.Fatalf("DetectLegacyVault err: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil info when no legacy file, got %+v", got)
	}
}

func TestDetectLegacyVault_ReturnsMetadata(t *testing.T) {
	svc, _, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, legacyPath, "alice", 2)

	info, err := svc.DetectLegacyVault()
	if err != nil {
		t.Fatalf("DetectLegacyVault err: %v", err)
	}
	if info == nil {
		t.Fatalf("expected info, got nil")
	}
	if info.Username != "alice" {
		t.Errorf("Username = %q, want %q", info.Username, "alice")
	}
	if info.Version != 2 {
		t.Errorf("Version = %d, want 2", info.Version)
	}
	if info.Path != legacyPath {
		t.Errorf("Path = %q, want %q", info.Path, legacyPath)
	}
	if info.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want positive", info.SizeBytes)
	}
}

// TestDetectLegacyVault_CorruptStillReported pins the design choice that a
// corrupt vault file is still reported by detection (so the UI can warn
// "found but unreadable") rather than silently hiding the user's data.
func TestDetectLegacyVault_CorruptStillReported(t *testing.T) {
	svc, _, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	if err := os.WriteFile(legacyPath, []byte("not json"), 0600); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	info, err := svc.DetectLegacyVault()
	if err != nil {
		t.Fatalf("DetectLegacyVault err: %v", err)
	}
	if info == nil {
		t.Fatalf("expected info even when corrupt, got nil")
	}
	if info.Username != "" {
		t.Errorf("expected empty username on corrupt vault, got %q", info.Username)
	}
}

// TestAdoptLegacyVault_NoCurrent installs the legacy bytes into the current
// path on a fresh-install setup (current vault doesn't exist yet).
func TestAdoptLegacyVault_NoCurrent(t *testing.T) {
	svc, currentPath, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, legacyPath, "alice", 2)

	if _, err := svc.AdoptLegacyVault(); err != nil {
		t.Fatalf("AdoptLegacyVault err: %v", err)
	}

	// Current path now holds the adopted vault.
	got, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	var v models.Vault
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("parse adopted vault: %v", err)
	}
	if v.Username != "alice" {
		t.Errorf("Username = %q, want alice", v.Username)
	}

	// Legacy file should be gone (cleanup so we don't keep prompting).
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("expected legacy path removed, stat err = %v", err)
	}
}

// TestAdoptLegacyVault_BacksUpExistingCurrent pins the orphaned-state fix:
// when both vaults exist, adoption archives the current one to a
// .replaced-<unix> sibling and installs the legacy bytes at the canonical
// path. The user's original vault must NEVER be silently destroyed.
func TestAdoptLegacyVault_BacksUpExistingCurrent(t *testing.T) {
	svc, currentPath, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, currentPath, "stub-user", 2)
	writeFakeVault(t, legacyPath, "alice", 2)

	if _, err := svc.AdoptLegacyVault(); err != nil {
		t.Fatalf("AdoptLegacyVault err: %v", err)
	}

	// Current now holds alice's vault.
	got, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	var v models.Vault
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("parse adopted vault: %v", err)
	}
	if v.Username != "alice" {
		t.Errorf("Username = %q, want alice", v.Username)
	}

	// A .replaced-<unix> backup of the prior current vault must exist next
	// to the canonical path. Find it by prefix match.
	dir := filepath.Dir(currentPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir current: %v", err)
	}
	var foundBackup string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "vault.osm.replaced-") {
			foundBackup = filepath.Join(dir, e.Name())
			break
		}
	}
	if foundBackup == "" {
		t.Fatalf("expected vault.osm.replaced-* backup, dir entries = %v", entries)
	}
	backupBytes, err := os.ReadFile(foundBackup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var bv models.Vault
	if err := json.Unmarshal(backupBytes, &bv); err != nil {
		t.Fatalf("parse backup: %v", err)
	}
	if bv.Username != "stub-user" {
		t.Errorf("backup Username = %q, want stub-user", bv.Username)
	}
}

// TestAdoptLegacyVault_RefusesUnlocked guards against adopting while a
// vault is unlocked — the in-memory state would no longer agree with the
// freshly-installed on-disk file, leaving the service confused.
func TestAdoptLegacyVault_RefusesUnlocked(t *testing.T) {
	svc, _, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, legacyPath, "alice", 2)

	svc.isUnlocked = true
	defer func() { svc.isUnlocked = false }()

	_, err := svc.AdoptLegacyVault()
	if err == nil {
		t.Fatal("expected error when adopting while unlocked")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Errorf("error %q should mention locking", err.Error())
	}
}

// TestAdoptLegacyVault_RejectsCorruptLegacy pins that adoption refuses to
// destroy a working current vault when the legacy file is junk. Read the
// legacy bytes, fail to parse → return an error WITHOUT having renamed the
// existing current vault.
func TestAdoptLegacyVault_RejectsCorruptLegacy(t *testing.T) {
	svc, currentPath, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, currentPath, "real-user", 2)
	if err := os.WriteFile(legacyPath, []byte("not json"), 0600); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}

	_, err := svc.AdoptLegacyVault()
	if err == nil {
		t.Fatal("expected error adopting corrupt legacy")
	}

	// Current vault must still hold real-user's data unchanged.
	got, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	var v models.Vault
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("parse current: %v", err)
	}
	if v.Username != "real-user" {
		t.Errorf("current vault was clobbered: Username = %q", v.Username)
	}
}

// TestAdoptLegacyVault_CarriesOverClientId pins the telemetry-continuity
// fix: when the legacy folder has a client.id and the current folder
// doesn't, adoption installs the legacy client.id at the current path so
// downstream events keep the same anonymous identity.
func TestAdoptLegacyVault_CarriesOverClientId(t *testing.T) {
	svc, currentPath, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, legacyPath, "alice", 2)

	legacyClientID := "B0000000-0000-0000-0000-000000000001"
	if err := os.WriteFile(filepath.Join(filepath.Dir(legacyPath), "client.id"), []byte(legacyClientID), 0600); err != nil {
		t.Fatalf("seed legacy client.id: %v", err)
	}

	result, err := svc.AdoptLegacyVault()
	if err != nil {
		t.Fatalf("AdoptLegacyVault: %v", err)
	}
	if !result.ClientIDCarried {
		t.Errorf("AdoptionResult.ClientIDCarried = false, want true")
	}

	got, err := os.ReadFile(filepath.Join(filepath.Dir(currentPath), "client.id"))
	if err != nil {
		t.Fatalf("read current client.id: %v", err)
	}
	if string(got) != legacyClientID {
		t.Errorf("current client.id = %q, want %q", got, legacyClientID)
	}
}

// TestAdoptLegacyVault_ArchivesExistingClientId pins that when both folders
// have a client.id, the current one is archived to client.id.replaced-<ts>
// before the legacy one is installed — same safety contract as vault.osm.
// Without this we'd silently lose the fresh-install client.id and any
// telemetry rows already keyed to it.
func TestAdoptLegacyVault_ArchivesExistingClientId(t *testing.T) {
	svc, currentPath, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, currentPath, "stub", 2)
	writeFakeVault(t, legacyPath, "alice", 2)

	currentClientID := "90000000-0000-0000-0000-000000000009"
	legacyClientID := "B0000000-0000-0000-0000-000000000001"
	if err := os.WriteFile(filepath.Join(filepath.Dir(currentPath), "client.id"), []byte(currentClientID), 0600); err != nil {
		t.Fatalf("seed current client.id: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(legacyPath), "client.id"), []byte(legacyClientID), 0600); err != nil {
		t.Fatalf("seed legacy client.id: %v", err)
	}

	if _, err := svc.AdoptLegacyVault(); err != nil {
		t.Fatalf("AdoptLegacyVault: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(filepath.Dir(currentPath), "client.id"))
	if err != nil {
		t.Fatalf("read current client.id: %v", err)
	}
	if string(got) != legacyClientID {
		t.Errorf("current client.id = %q, want %q (legacy)", got, legacyClientID)
	}

	// Archived current must exist as client.id.replaced-<ts>.
	dir := filepath.Dir(currentPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var found string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "client.id.replaced-") {
			found = filepath.Join(dir, e.Name())
			break
		}
	}
	if found == "" {
		t.Fatalf("expected client.id.replaced-* backup, entries = %v", entries)
	}
	backup, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != currentClientID {
		t.Errorf("backup = %q, want %q (current)", backup, currentClientID)
	}
}

// TestAdoptLegacyVault_RemovesLegacyDirEntirely pins the "no orphan
// reappears" contract. Before this change adoption only deleted the
// legacy vault.osm and rmdir'd the folder if it happened to be empty —
// any sidecar (client.id, logs/) would keep the dir alive and the next
// boot would re-detect it as a partial orphan. Now we RemoveAll.
func TestAdoptLegacyVault_RemovesLegacyDirEntirely(t *testing.T) {
	svc, _, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, legacyPath, "alice", 2)
	legacyDir := filepath.Dir(legacyPath)

	// Drop a sidecar + a logs/ subdir so the folder isn't empty after the
	// vault file moves.
	if err := os.WriteFile(filepath.Join(legacyDir, "client.id"), []byte("uuid"), 0600); err != nil {
		t.Fatalf("seed client.id: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(legacyDir, "logs"), 0700); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "logs", "app.log"), []byte("old events"), 0600); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	result, err := svc.AdoptLegacyVault()
	if err != nil {
		t.Fatalf("AdoptLegacyVault: %v", err)
	}
	if !result.LegacyDirRemoved {
		t.Errorf("AdoptionResult.LegacyDirRemoved = false, want true")
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("expected legacy dir removed, stat err = %v", err)
	}
}

// TestAdoptLegacyVault_NoLegacyClientIdIsFine pins that adoption succeeds
// when the legacy folder has only a vault.osm — historical OpenSmurfManager
// installs predating telemetry never wrote a client.id, and we shouldn't
// fail or warn for that.
func TestAdoptLegacyVault_NoLegacyClientIdIsFine(t *testing.T) {
	svc, _, legacyPath, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, legacyPath, "alice", 2)

	result, err := svc.AdoptLegacyVault()
	if err != nil {
		t.Fatalf("AdoptLegacyVault: %v", err)
	}
	if result.ClientIDCarried {
		t.Errorf("ClientIDCarried = true, want false (no legacy client.id existed)")
	}
}

// TestAdoptionResult_ArchivedCurrentReflectsExistence checks both branches
// of the archived_current attribute so the telemetry event is meaningful.
func TestAdoptionResult_ArchivedCurrentReflectsExistence(t *testing.T) {
	t.Run("no current → ArchivedCurrent=false", func(t *testing.T) {
		svc, _, legacyPath, cleanup := newServiceWithLegacy(t)
		defer cleanup()
		writeFakeVault(t, legacyPath, "alice", 2)

		result, err := svc.AdoptLegacyVault()
		if err != nil {
			t.Fatalf("AdoptLegacyVault: %v", err)
		}
		if result.ArchivedCurrent {
			t.Errorf("ArchivedCurrent = true, want false (no current vault existed)")
		}
	})

	t.Run("with current → ArchivedCurrent=true", func(t *testing.T) {
		svc, currentPath, legacyPath, cleanup := newServiceWithLegacy(t)
		defer cleanup()
		writeFakeVault(t, currentPath, "stub", 2)
		writeFakeVault(t, legacyPath, "alice", 2)

		result, err := svc.AdoptLegacyVault()
		if err != nil {
			t.Fatalf("AdoptLegacyVault: %v", err)
		}
		if !result.ArchivedCurrent {
			t.Errorf("ArchivedCurrent = false, want true")
		}
	})
}

// TestImportVaultFromPath_Success replaces the current vault with the bytes
// of an arbitrary user-supplied file. Unlike adoption, the source file is
// preserved so the user can keep their backup.
func TestImportVaultFromPath_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-import-*")
	if err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	currentPath := filepath.Join(tmpDir, "current", "vault.osm")
	importSrc := filepath.Join(tmpDir, "backup", "vault.osm")
	if err := os.MkdirAll(filepath.Dir(currentPath), 0700); err != nil {
		t.Fatalf("mkdir current: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(importSrc), 0700); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}

	writeFakeVault(t, currentPath, "stub", 2)
	writeFakeVault(t, importSrc, "alice", 2)

	svc := NewStorageServiceWithPath(currentPath)
	if err := svc.ImportVaultFromPath(importSrc); err != nil {
		t.Fatalf("ImportVaultFromPath: %v", err)
	}

	// Current path now holds alice's vault.
	got, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	var v models.Vault
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("parse imported: %v", err)
	}
	if v.Username != "alice" {
		t.Errorf("Username = %q, want alice", v.Username)
	}

	// Source file must be preserved (unlike adoption, which deletes the
	// legacy file). The user might want to keep their backup.
	if _, err := os.Stat(importSrc); err != nil {
		t.Errorf("import source was deleted: %v", err)
	}
}

func TestImportVaultFromPath_RejectsCorruptSource(t *testing.T) {
	svc, currentPath, _, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, currentPath, "real-user", 2)

	corrupt := filepath.Join(filepath.Dir(currentPath), "corrupt.osm")
	if err := os.WriteFile(corrupt, []byte("not json"), 0600); err != nil {
		t.Fatalf("seed corrupt: %v", err)
	}

	if err := svc.ImportVaultFromPath(corrupt); err == nil {
		t.Fatal("expected error importing corrupt source")
	}

	// Current vault must still hold real-user — refusing junk must not
	// have moved or deleted anything.
	got, _ := os.ReadFile(currentPath)
	var v models.Vault
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("parse current: %v", err)
	}
	if v.Username != "real-user" {
		t.Errorf("current was clobbered: Username = %q", v.Username)
	}
}

func TestImportVaultFromPath_RefusesUnlocked(t *testing.T) {
	svc, _, _, cleanup := newServiceWithLegacy(t)
	defer cleanup()

	src := filepath.Join(t.TempDir(), "vault.osm")
	writeFakeVault(t, src, "alice", 2)

	svc.isUnlocked = true
	defer func() { svc.isUnlocked = false }()

	err := svc.ImportVaultFromPath(src)
	if err == nil {
		t.Fatal("expected error when importing while unlocked")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Errorf("error %q should mention locking", err.Error())
	}
}

func TestImportVaultFromPath_RefusesSelfImport(t *testing.T) {
	svc, currentPath, _, cleanup := newServiceWithLegacy(t)
	defer cleanup()
	writeFakeVault(t, currentPath, "real", 2)

	err := svc.ImportVaultFromPath(currentPath)
	if err == nil {
		t.Fatal("expected error importing the active vault file onto itself")
	}
	if !strings.Contains(err.Error(), "active vault") {
		t.Errorf("error %q should mention the self-import case", err.Error())
	}
}

func TestImportVaultFromPath_NonExistent(t *testing.T) {
	svc, _, _, cleanup := newServiceWithLegacy(t)
	defer cleanup()

	missing := filepath.Join(t.TempDir(), "does-not-exist.osm")
	err := svc.ImportVaultFromPath(missing)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

// TestAdoptLegacyVault_AdoptedVaultUnlocks is the end-to-end safety check:
// create a real legacy vault using the existing CreateVaultWithRecoveryPhrase
// flow, point a fresh service at it via the override, run adoption, and
// confirm the resulting current vault unlocks with the original credentials.
// This pins that we copy the bytes verbatim — no re-encryption, no
// metadata loss.
func TestAdoptLegacyVault_AdoptedVaultUnlocks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osm-legacy-e2e-*")
	if err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	legacyPath := filepath.Join(tmpDir, "legacy", "vault.osm")
	currentPath := filepath.Join(tmpDir, "current", "vault.osm")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0700); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(currentPath), 0700); err != nil {
		t.Fatalf("mkdir current: %v", err)
	}

	// Build the real vault at the legacy path.
	legacySvc := NewStorageServiceWithPath(legacyPath)
	if _, err := legacySvc.CreateVaultWithRecoveryPhrase("alice", "secretpw", "hint"); err != nil {
		t.Fatalf("CreateVaultWithRecoveryPhrase: %v", err)
	}
	legacySvc.Lock()

	// Now stand up a fresh service at the current path with the legacy
	// override pointing at the just-created file. Adopt it.
	currentSvc := NewStorageServiceWithPath(currentPath)
	currentSvc.legacyVaultPathOverride = legacyPath

	if _, err := currentSvc.AdoptLegacyVault(); err != nil {
		t.Fatalf("AdoptLegacyVault: %v", err)
	}

	// The adopted vault must unlock with the original credentials.
	if err := currentSvc.Unlock("alice", "secretpw"); err != nil {
		t.Fatalf("Unlock after adopt: %v", err)
	}
	if !currentSvc.IsUnlocked() {
		t.Fatal("expected unlocked after adopt+Unlock")
	}
	if currentSvc.GetUsername() != "alice" {
		t.Errorf("Username = %q, want alice", currentSvc.GetUsername())
	}
}
