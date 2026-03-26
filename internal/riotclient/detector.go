package riotclient

import (
	"strings"
	"time"

	"OpenSmurfManager/internal/models"
)

// DetectedAccount represents an account detected via LCU
type DetectedAccount struct {
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
	// Try to connect to League client
	lcu, err := NewLeagueLCUClient()
	if err != nil {
		// Try Riot Client instead
		lcu, err = NewLCUClient()
		if err != nil {
			return nil, err
		}
	}

	// Get current summoner
	summoner, err := lcu.GetCurrentSummoner()
	if err != nil {
		// Fall back to Riot Client auth for just the Riot ID
		auth, err := lcu.GetRiotClientAuth()
		if err != nil {
			return nil, err
		}
		return &DetectedAccount{
			RiotID:     auth.GameName + "#" + auth.TagLine,
			GameName:   auth.GameName,
			TagLine:    auth.TagLine,
			PUUID:      auth.PUUID,
			DetectedAt: time.Now(),
		}, nil
	}

	// Build Riot ID
	riotID := summoner.GameName + "#" + summoner.TagLine

	detected := &DetectedAccount{
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

	// Get League ranks
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

	// Get TFT ranks
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

	// Update cached ranks
	account.CachedRanks = detected.Ranks

	// Update top masteries
	account.TopMasteries = detected.TopMasteries

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
