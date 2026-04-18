package riotclient

import (
	"strings"
	"time"

	"OpenSmurfManager/internal/models"
)

// DetectionError represents an error during account detection
type DetectionError struct {
	Code    string // "lockfile_not_found", "client_offline", "summoner_fetch_failed"
	Message string
	Retry   bool // Whether the caller should retry
}

func (e *DetectionError) Error() string {
	return e.Message
}

// DetectedAccount represents an account detected via LCU
type DetectedAccount struct {
	NetworkID     string // e.g., "riot", "epic"
	DisplayName   string // Primary label for the detected account
	Email         string // Network-specific email/username when available
	RiotID        string // e.g., "turkish aimer#doner"
	GameName      string
	TagLine       string
	PUUID         string
	SummonerLevel int
	Ranks         []models.CachedRank
	TopMasteries  []models.ChampionMastery
	DetectedAt    time.Time
}

// DetectAndFetchRanks connects to LCU, detects the signed-in account, and fetches ranks
func DetectAndFetchRanks() (*DetectedAccount, error) {
	// Try to connect to League client first
	lcu, err := NewLeagueLCUClient()
	if err != nil {
		// Try Riot Client instead
		lcu, err = NewLCUClient()
		if err != nil {
			return nil, &DetectionError{
				Code:    "client_offline",
				Message: "No Riot client running",
				Retry:   true,
			}
		}
	}

	// Get current summoner
	summoner, err := lcu.GetCurrentSummoner()
	if err != nil {
		// Fall back to Riot Client auth for just the Riot ID
		auth, err := lcu.GetRiotClientAuth()
		if err != nil {
			return nil, &DetectionError{
				Code:    "summoner_fetch_failed",
				Message: "Could not get account info",
				Retry:   true,
			}
		}
		return &DetectedAccount{
			NetworkID:   "riot",
			DisplayName: auth.GameName + "#" + auth.TagLine,
			RiotID:      auth.GameName + "#" + auth.TagLine,
			GameName:    auth.GameName,
			TagLine:     auth.TagLine,
			PUUID:       auth.PUUID,
			DetectedAt:  time.Now(),
		}, nil
	}

	// Build Riot ID
	riotID := summoner.GameName + "#" + summoner.TagLine

	detected := &DetectedAccount{
		NetworkID:     "riot",
		DisplayName:   riotID,
		RiotID:        riotID,
		GameName:      summoner.GameName,
		TagLine:       summoner.TagLine,
		PUUID:         summoner.PUUID,
		SummonerLevel: summoner.SummonerLevel,
		DetectedAt:    time.Now(),
	}

	// Fetch ranks
	leagueClient := NewLeagueClient(lcu)
	tftClient := NewTFTClient(lcu)

	now := time.Now()

	// Get League ranks (silent on failure - not all accounts have ranked data)
	if allRanks, err := leagueClient.GetAllRanks(); err == nil {
		for queueType, rank := range allRanks {
			detected.Ranks = append(detected.Ranks, models.CachedRank{
				GameID:      "lol",
				QueueType:   queueType,
				QueueName:   getQueueName(queueType),
				Tier:        rank.Tier,
				Division:    rank.Division,
				LP:          rank.LP,
				Wins:        rank.Wins,
				Losses:      rank.Losses,
				DisplayRank: rank.DisplayRank,
				LastUpdated: now,
			})
		}
	}

	// Get TFT ranks (silent on failure - not all accounts have TFT data)
	if tftRanks, err := tftClient.GetAllTFTRanks(); err == nil {
		for queueType, rank := range tftRanks {
			detected.Ranks = append(detected.Ranks, models.CachedRank{
				GameID:      "tft",
				QueueType:   queueType,
				QueueName:   getQueueName(queueType),
				Tier:        rank.Tier,
				Division:    rank.Division,
				LP:          rank.LP,
				Wins:        rank.Wins,
				Losses:      rank.Losses,
				DisplayRank: rank.DisplayRank,
				LastUpdated: now,
			})
		}
	}

	// Get top 3 champion masteries
	if masteries, err := leagueClient.GetTopChampionMasteries(3); err == nil {
		for _, m := range masteries {
			detected.TopMasteries = append(detected.TopMasteries, models.ChampionMastery{
				ChampionID:     m.ChampionID,
				ChampionName:   GetChampionName(m.ChampionID),
				ChampionLevel:  m.ChampionLevel,
				ChampionPoints: m.ChampionPoints,
				LastPlayTime:   m.LastPlayTime,
			})
		}
	}

	return detected, nil
}

// MatchAccountByRiotID finds an account in the list that matches the Riot ID
func MatchAccountByRiotID(accounts []models.Account, riotID string) *models.Account {
	riotID = strings.ToLower(riotID)

	for i := range accounts {
		if strings.ToLower(accounts[i].RiotID) == riotID {
			return &accounts[i]
		}
	}
	return nil
}

// MatchAccountByPUUID finds an account by PUUID
func MatchAccountByPUUID(accounts []models.Account, puuid string) *models.Account {
	for i := range accounts {
		if accounts[i].PUUID == puuid {
			return &accounts[i]
		}
	}
	return nil
}

// UpdateAccountRanks updates the cached ranks on an account
func UpdateAccountRanks(account *models.Account, detected *DetectedAccount) {
	// Update PUUID if not set
	if account.PUUID == "" && detected.PUUID != "" {
		account.PUUID = detected.PUUID
	}

	// Update Riot ID if not set
	if account.RiotID == "" && detected.RiotID != "" {
		account.RiotID = detected.RiotID
	}

	// Only update cached ranks if we got new data (don't wipe existing on API failure)
	if len(detected.Ranks) > 0 {
		account.CachedRanks = detected.Ranks
	}

	// Only update top masteries if we got new data
	if len(detected.TopMasteries) > 0 {
		account.TopMasteries = detected.TopMasteries
	}

	account.UpdatedAt = time.Now()
}

func getQueueName(queueType string) string {
	switch queueType {
	case "RANKED_SOLO_5x5":
		return "Solo/Duo"
	case "RANKED_FLEX_SR":
		return "Flex 5v5"
	case "RANKED_TFT":
		return "TFT Ranked"
	case "RANKED_TFT_DOUBLE_UP":
		return "Double Up"
	case "RANKED_TFT_TURBO":
		return "Hyper Roll"
	default:
		return queueType
	}
}
