package telemetry

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsDisabledAt_AbsentMeansEnabled pins the default state: with no
// marker file, telemetry is enabled (the install-time default).
func TestIsDisabledAt_AbsentMeansEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if isDisabledAt(path) {
		t.Errorf("isDisabledAt returned true for missing marker, want false")
	}
}

// TestIsDisabledAt_PresentMeansDisabled pins that the marker file being
// present is the sole signal for the opt-out state. Contents don't matter.
func TestIsDisabledAt_PresentMeansDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatalf("seed marker: %v", err)
	}
	if !isDisabledAt(path) {
		t.Errorf("isDisabledAt returned false with marker present, want true")
	}
}

// TestIsDisabledAt_DirectoryAsMarker pins a subtle edge case: if something
// creates a *directory* with the marker name, we still treat that as
// "present" (stat succeeds regardless of file type). Failing safe.
func TestIsDisabledAt_DirectoryAsMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatalf("seed marker dir: %v", err)
	}
	if !isDisabledAt(path) {
		t.Errorf("isDisabledAt returned false for marker-as-dir, want true")
	}
}

// TestSetDisabledAt_WritesMarker pins that disable=true creates the file.
func TestSetDisabledAt_WritesMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if err := setDisabledAt(path, true); err != nil {
		t.Fatalf("setDisabledAt(true): %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("marker not created: %v", err)
	}
}

// TestSetDisabledAt_RemovesMarker pins that disable=false removes an
// existing marker — the toggle-back-on path.
func TestSetDisabledAt_RemovesMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := setDisabledAt(path, false); err != nil {
		t.Fatalf("setDisabledAt(false): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("marker still present after disable=false: err=%v", err)
	}
}

// TestSetDisabledAt_IdempotentEnable pins that re-enabling when already
// enabled is a no-op (no error when the marker doesn't exist).
func TestSetDisabledAt_IdempotentEnable(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if err := setDisabledAt(path, false); err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if err := setDisabledAt(path, false); err != nil {
		t.Errorf("second enable (idempotent) returned error: %v", err)
	}
}

// TestSetDisabledAt_IdempotentDisable pins that re-disabling when already
// disabled is a no-op (re-creating the marker succeeds without error).
func TestSetDisabledAt_IdempotentDisable(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)
	if err := setDisabledAt(path, true); err != nil {
		t.Fatalf("first disable: %v", err)
	}
	if err := setDisabledAt(path, true); err != nil {
		t.Errorf("second disable (idempotent) returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("marker disappeared after second disable: %v", err)
	}
}

// TestSetDisabledAt_CreatesParentDir pins that disabling works even when
// the parent directory doesn't yet exist — a fresh install might have no
// appdir when this runs (e.g., installer hook preceding first launch).
func TestSetDisabledAt_CreatesParentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "deckstr", optOutFileName)
	if err := setDisabledAt(path, true); err != nil {
		t.Fatalf("setDisabledAt with missing parent: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("marker not created in nested path: %v", err)
	}
}

// TestSetDisabledAt_RoundTrip pins the full user flow: off, then back on,
// then off again — each transition reflected by isDisabledAt.
func TestSetDisabledAt_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), optOutFileName)

	if isDisabledAt(path) {
		t.Fatalf("initial state should be enabled")
	}
	if err := setDisabledAt(path, true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !isDisabledAt(path) {
		t.Fatalf("should be disabled after setDisabledAt(true)")
	}
	if err := setDisabledAt(path, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if isDisabledAt(path) {
		t.Fatalf("should be enabled after setDisabledAt(false)")
	}
	if err := setDisabledAt(path, true); err != nil {
		t.Fatalf("re-disable: %v", err)
	}
	if !isDisabledAt(path) {
		t.Fatalf("should be disabled after re-disable")
	}
}
