package riotclient

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestReadLeagueInstallDirsFromManifest(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "default install on C drive",
			content: `{
				"associated_client": {
					"C:/Riot Games/League of Legends/": "C:/Riot Games/Riot Client/RiotClientServices.exe"
				}
			}`,
			want: []string{"C:/Riot Games/League of Legends/"},
		},
		{
			name: "custom install on D drive",
			content: `{
				"associated_client": {
					"D:/Games/League of Legends/": "C:/Riot Games/Riot Client/RiotClientServices.exe"
				}
			}`,
			want: []string{"D:/Games/League of Legends/"},
		},
		{
			name: "deeply nested custom path",
			content: `{
				"associated_client": {
					"E:/My Games/Riot/LoL/League of Legends/": "C:/Riot Games/Riot Client/RiotClientServices.exe"
				}
			}`,
			want: []string{"E:/My Games/Riot/LoL/League of Legends/"},
		},
		{
			name: "League and Valorant both installed",
			content: `{
				"associated_client": {
					"C:/Riot Games/League of Legends/": "C:/Riot Games/Riot Client/RiotClientServices.exe",
					"C:/Riot Games/VALORANT/live/": "C:/Riot Games/Riot Client/RiotClientServices.exe"
				}
			}`,
			want: []string{"C:/Riot Games/League of Legends/"},
		},
		{
			name: "case-insensitive match on directory name",
			content: `{
				"associated_client": {
					"D:/Games/league of legends/": "C:/Riot Games/Riot Client/RiotClientServices.exe"
				}
			}`,
			want: []string{"D:/Games/league of legends/"},
		},
		{
			name: "only Valorant installed - no League dir returned",
			content: `{
				"associated_client": {
					"C:/Riot Games/VALORANT/live/": "C:/Riot Games/Riot Client/RiotClientServices.exe"
				}
			}`,
			want: nil,
		},
		{
			name:    "empty associated_client object",
			content: `{"associated_client": {}}`,
			want:    nil,
		},
		{
			name:    "missing associated_client key",
			content: `{"rc_default": "C:/Riot Games/Riot Client/RiotClientServices.exe"}`,
			want:    nil,
		},
		{
			name:    "malformed JSON returns nil without panic",
			content: `{not valid json`,
			want:    nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "RiotClientInstalls.json")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			got := readLeagueInstallDirsFromManifest(path)

			sort.Strings(got)
			sort.Strings(tc.want)

			if !equalSlices(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadLeagueInstallDirsFromManifest_MissingFile(t *testing.T) {
	got := readLeagueInstallDirsFromManifest(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

func TestRiotClientInstallsManifestPath_FallsBackWhenEnvUnset(t *testing.T) {
	t.Setenv("ProgramData", "")

	got := riotClientInstallsManifestPath()
	want := `C:\ProgramData\Riot Games\RiotClientInstalls.json`

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRiotClientInstallsManifestPath_UsesProgramDataEnv(t *testing.T) {
	t.Setenv("ProgramData", `D:\CustomProgramData`)

	got := riotClientInstallsManifestPath()
	want := filepath.Join(`D:\CustomProgramData`, "Riot Games", "RiotClientInstalls.json")

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func equalSlices(a, b []string) bool {
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
