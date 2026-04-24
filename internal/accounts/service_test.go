package accounts

import (
	"path/filepath"
	"testing"
	"time"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/storage"
)

// newTestService returns an AccountService backed by a freshly-created
// (and therefore unlocked) vault in a tmpdir. Mirrors the storage package
// test harness so the service layer is exercised against real storage
// rather than a stand-in — the interesting regressions live in the
// interaction, not in either side alone.
func newTestService(t *testing.T) *AccountService {
	t.Helper()
	vaultPath := filepath.Join(t.TempDir(), "vault.osm")
	svc := storage.NewStorageServiceWithPath(vaultPath)
	if err := svc.CreateVault("tester", "testpassword123"); err != nil {
		t.Fatalf("CreateVault: %v", err)
	}
	return NewAccountService(svc)
}

// seedAccount is a minimal valid account — tests override fields as needed
// without restating the whole struct each time.
func seedAccount() models.Account {
	return models.Account{
		DisplayName: "MainAccount",
		Username:    "mainuser",
		Password:    "secret",
		NetworkID:   "riot",
		Games:       []string{"lol"},
		Tags:        []string{"smurf"},
	}
}

// TestCreate_GeneratesIDAndTimestamps pins that Create assigns a fresh
// UUID plus CreatedAt/UpdatedAt, without the caller providing them.
func TestCreate_GeneratesIDAndTimestamps(t *testing.T) {
	svc := newTestService(t)
	before := time.Now().Add(-time.Second)

	created, err := svc.Create(seedAccount())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Error("created.ID is empty — UUID not assigned")
	}
	if !created.CreatedAt.After(before) {
		t.Errorf("CreatedAt = %v, want after %v", created.CreatedAt, before)
	}
	if !created.UpdatedAt.After(before) {
		t.Errorf("UpdatedAt = %v, want after %v", created.UpdatedAt, before)
	}
}

// TestCreate_RejectsEmptyUsername pins the single validation rule in
// Create: Username must be non-empty. Display name, tags, etc. are
// optional.
func TestCreate_RejectsEmptyUsername(t *testing.T) {
	svc := newTestService(t)
	a := seedAccount()
	a.Username = ""
	if _, err := svc.Create(a); err != ErrInvalidAccount {
		t.Errorf("Create(empty username) err = %v, want %v", err, ErrInvalidAccount)
	}
}

// TestCreate_InitializesNilSlices pins that nil Tags/Games/CachedRanks
// become empty slices on persist. Prevents downstream code from having
// to do nil-vs-empty checks, and avoids JSON emitting `"tags":null`
// which the frontend would need to defend against.
func TestCreate_InitializesNilSlices(t *testing.T) {
	svc := newTestService(t)
	a := seedAccount()
	a.Tags = nil
	a.Games = nil
	a.CachedRanks = nil

	created, err := svc.Create(a)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Tags == nil {
		t.Error("Tags is nil after Create, want empty slice")
	}
	if created.Games == nil {
		t.Error("Games is nil after Create, want empty slice")
	}
	if created.CachedRanks == nil {
		t.Error("CachedRanks is nil after Create, want empty slice")
	}
}

// TestGetByID covers both the hit and miss paths — a found account is
// returned by value (copy), and a miss surfaces ErrAccountNotFound rather
// than (nil, nil).
func TestGetByID(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.Create(seedAccount())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID(hit): %v", err)
	}
	if got.ID != created.ID || got.Username != created.Username {
		t.Errorf("GetByID returned wrong account: got %+v, want %+v", got, created)
	}

	if _, err := svc.GetByID("no-such-id"); err != ErrAccountNotFound {
		t.Errorf("GetByID(miss) err = %v, want %v", err, ErrAccountNotFound)
	}
}

// TestGetByNetwork_FiltersByNetworkID pins that accounts are partitioned
// by NetworkID and only the matching ones are returned.
func TestGetByNetwork_FiltersByNetworkID(t *testing.T) {
	svc := newTestService(t)

	riotAcc := seedAccount() // NetworkID = "riot"
	epicAcc := seedAccount()
	epicAcc.NetworkID = "epic"
	epicAcc.Username = "epicuser"

	if _, err := svc.Create(riotAcc); err != nil {
		t.Fatalf("Create riot: %v", err)
	}
	if _, err := svc.Create(epicAcc); err != nil {
		t.Fatalf("Create epic: %v", err)
	}

	riots, err := svc.GetByNetwork("riot")
	if err != nil {
		t.Fatalf("GetByNetwork: %v", err)
	}
	if len(riots) != 1 || riots[0].NetworkID != "riot" {
		t.Errorf("GetByNetwork(riot) = %+v, want one riot account", riots)
	}

	nones, err := svc.GetByNetwork("steam")
	if err != nil {
		t.Fatalf("GetByNetwork(steam): %v", err)
	}
	if len(nones) != 0 {
		t.Errorf("GetByNetwork(steam) = %+v, want empty", nones)
	}
}

// TestGetByTag_CaseInsensitive pins the documented tag-match behavior:
// "Smurf", "smurf", and "SMURF" all match the same account. The
// installer/wizard surface tag case inconsistently (autocomplete preserves
// casing) so this guarantee matters.
func TestGetByTag_CaseInsensitive(t *testing.T) {
	svc := newTestService(t)
	a := seedAccount()
	a.Tags = []string{"Smurf"}
	if _, err := svc.Create(a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, probe := range []string{"smurf", "SMURF", "Smurf", "sMuRf"} {
		got, err := svc.GetByTag(probe)
		if err != nil {
			t.Fatalf("GetByTag(%q): %v", probe, err)
		}
		if len(got) != 1 {
			t.Errorf("GetByTag(%q) = %d hits, want 1", probe, len(got))
		}
	}
}

// TestSearch covers the three fields Search looks at — DisplayName,
// Username, Notes — all case-insensitive. Ensures a refactor that (say)
// moved to an index doesn't silently drop any of them.
func TestSearch(t *testing.T) {
	svc := newTestService(t)

	a1 := seedAccount()
	a1.DisplayName = "Alpha Player"
	a1.Username = "alice"
	a1.Notes = "primary smurf"

	a2 := seedAccount()
	a2.DisplayName = "Beta"
	a2.Username = "bob123"
	a2.Notes = "unranked"

	if _, err := svc.Create(a1); err != nil {
		t.Fatalf("Create a1: %v", err)
	}
	if _, err := svc.Create(a2); err != nil {
		t.Fatalf("Create a2: %v", err)
	}

	cases := []struct {
		query   string
		wantLen int
	}{
		{"alpha", 1},     // DisplayName substring (case-insensitive)
		{"BOB", 1},       // Username substring (case-insensitive)
		{"UNRANKED", 1},  // Notes substring (case-insensitive)
		{"smurf", 1},     // Notes substring — only a1 has it
		{"player", 1},    // DisplayName partial
		{"ghost", 0},     // No match
	}
	for _, tc := range cases {
		got, err := svc.Search(tc.query)
		if err != nil {
			t.Fatalf("Search(%q): %v", tc.query, err)
		}
		if len(got) != tc.wantLen {
			t.Errorf("Search(%q) = %d hits, want %d", tc.query, len(got), tc.wantLen)
		}
	}
}

// TestUpdate_PreservesCreatedAt pins the load-bearing invariant that
// Update never rewrites CreatedAt. A bug here silently resets the "age"
// of every account whenever a user edits it — hard to spot, painful to
// undo.
func TestUpdate_PreservesCreatedAt(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.Create(seedAccount())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	originalCreatedAt := created.CreatedAt

	// Sleep briefly so UpdatedAt is distinguishable from the original.
	time.Sleep(10 * time.Millisecond)

	created.DisplayName = "Renamed"
	updated, err := svc.Update(*created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if !updated.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("CreatedAt changed: was %v, now %v", originalCreatedAt, updated.CreatedAt)
	}
	if !updated.UpdatedAt.After(originalCreatedAt) {
		t.Errorf("UpdatedAt not advanced: CreatedAt=%v UpdatedAt=%v", originalCreatedAt, updated.UpdatedAt)
	}
	if updated.DisplayName != "Renamed" {
		t.Errorf("DisplayName = %q, want Renamed", updated.DisplayName)
	}
}

// TestUpdate_RejectsInvalidInput covers the two pre-flight checks:
// empty ID and empty Username. Both are caller-provided and should
// short-circuit before hitting storage.
func TestUpdate_RejectsInvalidInput(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.Update(models.Account{ID: "", Username: "u"}); err != ErrInvalidAccount {
		t.Errorf("Update(empty ID) err = %v, want %v", err, ErrInvalidAccount)
	}
	if _, err := svc.Update(models.Account{ID: "abc", Username: ""}); err != ErrInvalidAccount {
		t.Errorf("Update(empty username) err = %v, want %v", err, ErrInvalidAccount)
	}
}

// TestUpdate_NotFound pins that updating a nonexistent ID returns
// ErrAccountNotFound rather than silently creating it.
func TestUpdate_NotFound(t *testing.T) {
	svc := newTestService(t)
	a := seedAccount()
	a.ID = "no-such-id"
	if _, err := svc.Update(a); err != ErrAccountNotFound {
		t.Errorf("Update(missing) err = %v, want %v", err, ErrAccountNotFound)
	}
}

// TestDelete covers happy + miss paths. Also asserts the survivor set
// so we catch bugs where the wrong index is spliced out (off-by-one on
// append(s[:i], s[i+1:]...)).
func TestDelete(t *testing.T) {
	svc := newTestService(t)

	a1, err := svc.Create(seedAccount())
	if err != nil {
		t.Fatalf("Create a1: %v", err)
	}
	second := seedAccount()
	second.Username = "second"
	a2, err := svc.Create(second)
	if err != nil {
		t.Fatalf("Create a2: %v", err)
	}

	if err := svc.Delete(a1.ID); err != nil {
		t.Fatalf("Delete(a1): %v", err)
	}

	remaining, err := svc.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != a2.ID {
		t.Errorf("after Delete(a1), remaining = %+v, want only a2", remaining)
	}

	if err := svc.Delete("no-such-id"); err != ErrAccountNotFound {
		t.Errorf("Delete(missing) err = %v, want %v", err, ErrAccountNotFound)
	}
}

// TestUpdateRank_AddsThenUpdates exercises the split path inside
// UpdateRank: first call inserts a new CachedRank entry for the (account,
// game) pair; second call with the same pair mutates in place rather than
// appending a duplicate.
func TestUpdateRank_AddsThenUpdates(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.Create(seedAccount())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.UpdateRank(created.ID, "lol", "Gold IV", 42); err != nil {
		t.Fatalf("UpdateRank insert: %v", err)
	}
	got, _ := svc.GetByID(created.ID)
	if len(got.CachedRanks) != 1 {
		t.Fatalf("after first UpdateRank: %d ranks, want 1", len(got.CachedRanks))
	}
	if got.CachedRanks[0].DisplayRank != "Gold IV" || got.CachedRanks[0].LP != 42 {
		t.Errorf("rank = %+v, want Gold IV @ 42 LP", got.CachedRanks[0])
	}

	if err := svc.UpdateRank(created.ID, "lol", "Platinum II", 15); err != nil {
		t.Fatalf("UpdateRank mutate: %v", err)
	}
	got, _ = svc.GetByID(created.ID)
	if len(got.CachedRanks) != 1 {
		t.Errorf("after second UpdateRank: %d ranks, want 1 (mutated in place)", len(got.CachedRanks))
	}
	if got.CachedRanks[0].DisplayRank != "Platinum II" || got.CachedRanks[0].LP != 15 {
		t.Errorf("rank = %+v, want Platinum II @ 15 LP", got.CachedRanks[0])
	}
}

// TestUpdateRank_NotFound pins that rank updates against a nonexistent
// account surface ErrAccountNotFound rather than silently doing nothing.
func TestUpdateRank_NotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.UpdateRank("no-such-id", "lol", "Gold", 0); err != ErrAccountNotFound {
		t.Errorf("UpdateRank(missing) err = %v, want %v", err, ErrAccountNotFound)
	}
}

// TestAddTag_DedupesCaseInsensitive pins the documented tag-set invariant:
// tag membership is case-insensitive. A second add of the same tag (any
// casing) is a silent no-op. A genuinely new tag is appended preserving
// the caller's casing.
func TestAddTag_DedupesCaseInsensitive(t *testing.T) {
	svc := newTestService(t)

	// Baseline: the default vault pre-seeds ["main", "smurf"].
	baseline, err := svc.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags baseline: %v", err)
	}
	baselineLen := len(baseline)

	// Re-adding any casing of a pre-seeded tag must be a no-op.
	for _, dup := range []string{"Alt", "alt", "ALT", "Main", "MAIN"} {
		if err := svc.AddTag(dup); err != nil {
			t.Fatalf("AddTag(%q) dup: %v", dup, err)
		}
	}
	afterDups, err := svc.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags post-dups: %v", err)
	}
	if len(afterDups) != baselineLen {
		t.Errorf("tags grew from %d to %d after only-duplicate adds: %v", baselineLen, len(afterDups), afterDups)
	}

	// A genuinely new tag gets appended and keeps its casing.
	if err := svc.AddTag("Ranked"); err != nil {
		t.Fatalf("AddTag new: %v", err)
	}
	final, err := svc.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags final: %v", err)
	}
	if len(final) != baselineLen+1 {
		t.Errorf("tags = %v, expected one more than baseline (%d)", final, baselineLen)
	}
	found := false
	for _, tag := range final {
		if tag == "Ranked" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tags = %v, want to contain %q with preserved casing", final, "Ranked")
	}

	// Sanity: case-insensitive dedup against the *newly added* tag too.
	if err := svc.AddTag("RANKED"); err != nil {
		t.Fatalf("AddTag dup of new tag: %v", err)
	}
	afterRanked, err := svc.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags post-ranked-dup: %v", err)
	}
	if len(afterRanked) != len(final) {
		t.Errorf("RANKED was added despite existing Ranked: %v", afterRanked)
	}
}
