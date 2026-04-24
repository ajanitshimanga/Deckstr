package models

import "testing"

// TestRocketLeagueUnderEpicAndSteam pins the cross-platform contract that
// Rocket League ships under both storefronts. A regression here would mean
// accounts from one store silently disappear from the picker.
func TestRocketLeagueUnderEpicAndSteam(t *testing.T) {
	networks := DefaultGameNetworks()

	// Build a lookup so the assertions read linearly.
	byNetwork := map[string]*Game{}
	for i := range networks {
		for j := range networks[i].Games {
			g := &networks[i].Games[j]
			if g.ID == "rl" {
				byNetwork[networks[i].ID] = g
			}
		}
	}

	for _, networkID := range []string{"epic", "steam"} {
		g, ok := byNetwork[networkID]
		if !ok {
			t.Errorf("Rocket League missing under network %q", networkID)
			continue
		}
		if g.NetworkID != networkID {
			t.Errorf("network %q: Rocket League game.NetworkID = %q, want %q",
				networkID, g.NetworkID, networkID)
		}
		if len(g.GameProcesses.Windows) == 0 {
			t.Errorf("network %q: Rocket League must declare at least one Windows game process",
				networkID)
		}
		if len(g.ClientProcess.Windows) == 0 {
			t.Errorf("network %q: launcher must declare at least one Windows client process",
				networkID)
		}
	}
}

// TestRocketLeagueSharesGameProcessesAcrossStores pins the invariant that
// Psyonix ships the same RocketLeague.exe regardless of storefront. If the
// binary ever diverges we want an explicit change site, not a silent drift.
func TestRocketLeagueSharesGameProcessesAcrossStores(t *testing.T) {
	var epicRL, steamRL []string
	for _, n := range DefaultGameNetworks() {
		for _, g := range n.Games {
			if g.ID != "rl" {
				continue
			}
			switch n.ID {
			case "epic":
				epicRL = g.GameProcesses.Windows
			case "steam":
				steamRL = g.GameProcesses.Windows
			}
		}
	}
	if len(epicRL) == 0 || len(steamRL) == 0 {
		t.Fatalf("expected Rocket League under both stores, got epic=%v steam=%v", epicRL, steamRL)
	}
	if !equalStringSlice(epicRL, steamRL) {
		t.Errorf("Rocket League game processes diverged across stores: epic=%v, steam=%v",
			epicRL, steamRL)
	}
}

// TestSharedAccountFlag pins the platform-truth invariants the wizard relies
// on: Riot binds one login to every Riot game (LoL/TFT/Valorant); Steam and
// Epic do not. A regression here would cause the wizard to either over-tag
// Steam/Epic accounts with sibling games or under-tag Riot accounts.
func TestSharedAccountFlag(t *testing.T) {
	want := map[string]bool{
		"riot":  true,
		"epic":  false,
		"steam": false,
	}
	for _, n := range DefaultGameNetworks() {
		expected, ok := want[n.ID]
		if !ok {
			continue
		}
		if n.SharedAccount != expected {
			t.Errorf("network %q: SharedAccount = %v, want %v", n.ID, n.SharedAccount, expected)
		}
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
