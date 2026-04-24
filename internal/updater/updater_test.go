package updater

import (
	"runtime"
	"testing"
)

// TestIsNewerVersion covers the semver-comparison logic used to decide
// whether to prompt the user about an update. Written as a table because
// every case is "inputs → one bool" and the failure modes are all the
// same shape.
func TestIsNewerVersion(t *testing.T) {
	u := &Updater{}
	cases := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		// Happy paths
		{"patch bump", "1.3.2", "1.3.1", true},
		{"minor bump", "1.4.0", "1.3.9", true},
		{"major bump", "2.0.0", "1.9.9", true},

		// Equality and regress — never prompt
		{"equal", "1.3.1", "1.3.1", false},
		{"older latest", "1.3.0", "1.3.1", false},
		{"older major", "1.0.0", "2.0.0", false},

		// Dev builds — never prompt, even if upstream is ahead
		{"dev skips prompt", "1.3.1", "dev", false},
		{"empty current skips prompt", "1.3.1", "", false},

		// Length mismatches. Simple comparison treats a longer tag as newer
		// when the common prefix matches — this is what ships today, and
		// pinning it prevents accidental behavior changes in the comparator.
		{"longer latest wins after equal prefix", "1.3.1.1", "1.3.1", true},
		{"shorter latest loses after equal prefix", "1.3.1", "1.3.1.1", false},

		// String compare — "10" vs "9" — flags a known limitation of the
		// current comparator. Documented here so the next rewrite notices.
		// "10" < "9" lexically, so the comparator reports no update even
		// though 1.10.0 > 1.9.0 numerically. If this ever changes (semver
		// lib), delete this case rather than "fixing" it silently.
		{"lexical compare misranks 10 vs 9", "1.10.0", "1.9.0", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := u.isNewerVersion(tc.latest, tc.current)
			if got != tc.want {
				t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
			}
		})
	}
}

// TestGetAssetName pins the platform-to-asset-name mapping. The installer
// name must match what's actually published on the GitHub release for the
// current platform; if this drifts, auto-update silently breaks.
func TestGetAssetName(t *testing.T) {
	u := &Updater{}
	got := u.getAssetName()

	var want string
	switch runtime.GOOS {
	case "windows":
		want = "Setup"
	case "darwin":
		want = ".dmg"
	case "linux":
		want = ".AppImage"
	default:
		want = ""
	}

	if got != want {
		t.Errorf("getAssetName() on %s = %q, want %q", runtime.GOOS, got, want)
	}
}

// TestNewUpdater_UsesPackageDefaults pins that NewUpdater wires up the
// build-time-injected owner/repo/version constants rather than hardcoded
// values. A regression here would silently break update checks on prod
// builds where ldflags set these.
func TestNewUpdater_UsesPackageDefaults(t *testing.T) {
	u := NewUpdater()
	if u.owner != GitHubOwner {
		t.Errorf("owner = %q, want package var %q", u.owner, GitHubOwner)
	}
	if u.repo != GitHubRepo {
		t.Errorf("repo = %q, want package var %q", u.repo, GitHubRepo)
	}
	if u.current != Version {
		t.Errorf("current = %q, want package var %q", u.current, Version)
	}
	if u.client == nil {
		t.Error("client is nil — HTTP requests will panic")
	}
	if u.GetCurrentVersion() != Version {
		t.Errorf("GetCurrentVersion() = %q, want %q", u.GetCurrentVersion(), Version)
	}
}
