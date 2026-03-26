package models

import "time"

// GameNetwork represents a gaming platform (e.g., Riot Games, Steam)
type GameNetwork struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Games []Game `json:"games"`
}

// Game represents a specific game within a network
type Game struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	NetworkID     string `json:"networkId"`
	ClientProcess string `json:"clientProcess"` // Process name to detect
	ClientTitle   string `json:"clientTitle"`   // Window title pattern
}

// Account represents a gaming account with encrypted credentials
type Account struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"displayName"`
	Username    string     `json:"username"` // Stored encrypted
	Password    string     `json:"password"` // Stored encrypted
	NetworkID   string     `json:"networkId"`
	Tags        []string   `json:"tags"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	// Riot-specific fields
	RiotID   string `json:"riotId,omitempty"`   // e.g., "turkish aimer#doner"
	PUUID    string `json:"puuid,omitempty"`    // Cached PUUID for API calls
	Region   string `json:"region,omitempty"`   // e.g., "na1", "euw1"

	// Game filters - which games this account is used for
	Games []string `json:"games,omitempty"` // e.g., ["lol", "tft", "valorant"]

	// Cached rank data (persists even when not signed in)
	CachedRanks []CachedRank `json:"cachedRanks,omitempty"`

	// Top champion/agent masteries (top 3)
	TopMasteries []ChampionMastery `json:"topMasteries,omitempty"`
}

// CachedRank stores cached rank data for a specific queue
type CachedRank struct {
	GameID      string    `json:"gameId"`      // "lol", "tft", "valorant"
	QueueType   string    `json:"queueType"`   // "RANKED_SOLO_5x5", "RANKED_TFT", etc.
	QueueName   string    `json:"queueName"`   // "Solo/Duo", "TFT Ranked", etc.
	Tier        string    `json:"tier"`        // "GOLD", "DIAMOND", etc.
	Division    string    `json:"division"`    // "I", "II", "III", "IV"
	LP          int       `json:"lp"`          // League Points
	Wins        int       `json:"wins"`
	Losses      int       `json:"losses"`
	DisplayRank string    `json:"displayRank"` // "Gold II 62 LP"
	LastUpdated time.Time `json:"lastUpdated"`
}

// ChampionMastery stores mastery data for a champion
type ChampionMastery struct {
	ChampionID     int    `json:"championId"`
	ChampionName   string `json:"championName"`   // Resolved name (e.g., "Yasuo")
	ChampionLevel  int    `json:"championLevel"`  // 1-7
	ChampionPoints int    `json:"championPoints"` // Total mastery points
	LastPlayTime   int64  `json:"lastPlayTime"`   // Unix timestamp
}

// RankInfo stores rank data for a specific game (legacy, kept for compatibility)
type RankInfo struct {
	GameID      string    `json:"gameId"`
	Rank        string    `json:"rank"` // e.g., "Diamond 2", "Immortal 1"
	LP          int       `json:"lp"`   // League Points (optional)
	LastUpdated time.Time `json:"lastUpdated"`
}

// ParseRiotID splits a Riot ID into gameName and tagLine
func ParseRiotID(riotID string) (gameName, tagLine string, ok bool) {
	parts := splitRiotID(riotID)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func splitRiotID(riotID string) []string {
	// Find the last # to handle game names with special characters
	idx := -1
	for i := len(riotID) - 1; i >= 0; i-- {
		if riotID[i] == '#' {
			idx = i
			break
		}
	}
	if idx == -1 || idx == 0 || idx == len(riotID)-1 {
		return nil
	}
	return []string{riotID[:idx], riotID[idx+1:]}
}

// Vault represents the encrypted data store
type Vault struct {
	Version       int       `json:"version"`
	Username      string    `json:"username"`      // Account username (for future cloud auth)
	PasswordHint  string    `json:"passwordHint"`  // Optional hint displayed on lock screen (not encrypted)
	Salt          string    `json:"salt"`          // Base64 encoded salt for key derivation
	Nonce         string    `json:"nonce"`         // Base64 encoded nonce for encryption
	EncryptedData string    `json:"encryptedData"` // Base64 encoded encrypted JSON
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// VaultData represents the decrypted vault contents
type VaultData struct {
	Accounts     []Account     `json:"accounts"`
	GameNetworks []GameNetwork `json:"gameNetworks"`
	Tags         []string      `json:"tags"` // All available tags
	Settings     Settings      `json:"settings"`
}

// Settings represents user preferences
type Settings struct {
	AutoLockTimeout int  `json:"autoLockTimeout"` // Minutes, 0 = disabled
	StartWithSystem bool `json:"startWithSystem"`
	MinimizeToTray  bool `json:"minimizeToTray"`
	DarkMode        bool `json:"darkMode"`

	// BYOK: User's own Riot API key (stored encrypted in vault)
	RiotAPIKey string `json:"riotApiKey,omitempty"`

	// Default region for API calls
	DefaultRegion string `json:"defaultRegion,omitempty"` // e.g., "na1", "euw1"
}

// DefaultGameNetworks returns pre-populated game networks
func DefaultGameNetworks() []GameNetwork {
	return []GameNetwork{
		{
			ID:   "riot",
			Name: "Riot Games",
			Games: []Game{
				{
					ID:            "lol",
					Name:          "League of Legends",
					NetworkID:     "riot",
					ClientProcess: "RiotClientUx.exe",
					ClientTitle:   "Riot Client",
				},
				{
					ID:            "valorant",
					Name:          "Valorant",
					NetworkID:     "riot",
					ClientProcess: "RiotClientUx.exe",
					ClientTitle:   "Riot Client",
				},
				{
					ID:            "tft",
					Name:          "Teamfight Tactics",
					NetworkID:     "riot",
					ClientProcess: "RiotClientUx.exe",
					ClientTitle:   "Riot Client",
				},
			},
		},
	}
}

// DefaultSettings returns default user settings
func DefaultSettings() Settings {
	return Settings{
		AutoLockTimeout: 5,
		StartWithSystem: false,
		MinimizeToTray:  true,
		DarkMode:        true,
	}
}

// NewVaultData creates a new empty vault with defaults
func NewVaultData() VaultData {
	return VaultData{
		Accounts:     []Account{},
		GameNetworks: DefaultGameNetworks(),
		Tags:         []string{"main", "smurf"},
		Settings:     DefaultSettings(),
	}
}
