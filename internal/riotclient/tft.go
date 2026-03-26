package riotclient

import (
	"fmt"
)

// TFTClient handles Teamfight Tactics specific API calls
type TFTClient struct {
	lcu *LCUClient
}

// NewTFTClient creates a new TFT client
func NewTFTClient(lcu *LCUClient) *TFTClient {
	return &TFTClient{lcu: lcu}
}

// TFTRankInfo contains formatted rank information for TFT
type TFTRankInfo struct {
	QueueType   string // RANKED_TFT, RANKED_TFT_DOUBLE_UP, RANKED_TFT_TURBO, etc.
	Tier        string // IRON, BRONZE, SILVER, GOLD, PLATINUM, EMERALD, DIAMOND, MASTER, GRANDMASTER, CHALLENGER
	Division    string // I, II, III, IV
	LP          int
	Wins        int
	Losses      int
	Provisional bool
	DisplayRank string // e.g., "Diamond II 45 LP"
}

// TFT Queue Types
const (
	TFTRanked         = "RANKED_TFT"
	TFTDoubleUp       = "RANKED_TFT_DOUBLE_UP"
	TFTTurbo          = "RANKED_TFT_TURBO"
	TFTHyperRoll      = "RANKED_TFT_TURBO" // Alias for Hyper Roll
)

// GetRankedStats gets the TFT ranked stats for the current summoner
func (c *TFTClient) GetRankedStats() (*RankedData, error) {
	summoner, err := c.lcu.GetCurrentSummoner()
	if err != nil {
		return nil, fmt.Errorf("failed to get current summoner: %w", err)
	}

	var ranked RankedData
	endpoint := fmt.Sprintf("/lol-ranked/v1/ranked-stats/%s", summoner.PUUID)
	err = c.lcu.GetJSON(endpoint, &ranked)
	if err != nil {
		return nil, fmt.Errorf("failed to get ranked stats: %w", err)
	}

	return &ranked, nil
}

// GetTFTRank gets the main TFT ranked queue rank
func (c *TFTClient) GetTFTRank() (*TFTRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	if stats, ok := ranked.QueueMap["RANKED_TFT"]; ok {
		return formatTFTRankInfo("RANKED_TFT", &stats), nil
	}

	return nil, fmt.Errorf("no TFT ranked data found")
}

// GetDoubleUpRank gets the TFT Double Up ranked queue rank
func (c *TFTClient) GetDoubleUpRank() (*TFTRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	if stats, ok := ranked.QueueMap["RANKED_TFT_DOUBLE_UP"]; ok {
		return formatTFTRankInfo("RANKED_TFT_DOUBLE_UP", &stats), nil
	}

	return nil, fmt.Errorf("no TFT Double Up ranked data found")
}

// GetHyperRollRating gets the TFT Hyper Roll rating
func (c *TFTClient) GetHyperRollRating() (*TFTRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	if stats, ok := ranked.QueueMap["RANKED_TFT_TURBO"]; ok {
		return formatTFTHyperRollInfo(&stats), nil
	}

	return nil, fmt.Errorf("no TFT Hyper Roll data found")
}

// GetAllTFTRanks gets all TFT ranked queue stats
func (c *TFTClient) GetAllTFTRanks() (map[string]*TFTRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	ranks := make(map[string]*TFTRankInfo)

	// Filter for TFT queues only
	tftQueues := []string{"RANKED_TFT", "RANKED_TFT_DOUBLE_UP", "RANKED_TFT_TURBO"}

	for _, queueType := range tftQueues {
		if stats, ok := ranked.QueueMap[queueType]; ok {
			if stats.Tier != "" && stats.Tier != "NONE" {
				if queueType == "RANKED_TFT_TURBO" {
					ranks[queueType] = formatTFTHyperRollInfo(&stats)
				} else {
					ranks[queueType] = formatTFTRankInfo(queueType, &stats)
				}
			} else if queueType == "RANKED_TFT_TURBO" && stats.RatedRating > 0 {
				// Hyper Roll uses rating instead of tier
				ranks[queueType] = formatTFTHyperRollInfo(&stats)
			}
		}
	}

	return ranks, nil
}

func formatTFTRankInfo(queueType string, stats *RankedStats) *TFTRankInfo {
	info := &TFTRankInfo{
		QueueType:   queueType,
		Tier:        stats.Tier,
		Division:    stats.Division,
		LP:          stats.LeaguePoints,
		Wins:        stats.Wins,
		Losses:      stats.Losses,
		Provisional: stats.IsProvisional,
	}

	// Format display rank
	if stats.Tier == "" || stats.Tier == "NONE" {
		info.DisplayRank = "Unranked"
	} else if stats.IsProvisional {
		info.DisplayRank = fmt.Sprintf("Provisional (%d games left)", stats.ProvisionalGamesRemaining)
	} else if stats.Tier == "MASTER" || stats.Tier == "GRANDMASTER" || stats.Tier == "CHALLENGER" {
		info.DisplayRank = fmt.Sprintf("%s %d LP", capitalizeFirst(stats.Tier), stats.LeaguePoints)
	} else {
		info.DisplayRank = fmt.Sprintf("%s %s %d LP", capitalizeFirst(stats.Tier), stats.Division, stats.LeaguePoints)
	}

	// Add queue name suffix for clarity
	switch queueType {
	case "RANKED_TFT_DOUBLE_UP":
		info.DisplayRank += " (Double Up)"
	}

	return info
}

func formatTFTHyperRollInfo(stats *RankedStats) *TFTRankInfo {
	info := &TFTRankInfo{
		QueueType: "RANKED_TFT_TURBO",
		LP:        stats.RatedRating,
		Wins:      stats.Wins,
		Losses:    stats.Losses,
	}

	// Hyper Roll uses rating tiers instead of traditional ranks
	// Gray (0-1399), Green (1400-2599), Blue (2600-3599), Purple (3600-4599), Hyper (4600+)
	rating := stats.RatedRating
	switch {
	case rating >= 4600:
		info.Tier = "HYPER"
		info.DisplayRank = fmt.Sprintf("Hyper %d", rating)
	case rating >= 3600:
		info.Tier = "PURPLE"
		info.DisplayRank = fmt.Sprintf("Purple %d", rating)
	case rating >= 2600:
		info.Tier = "BLUE"
		info.DisplayRank = fmt.Sprintf("Blue %d", rating)
	case rating >= 1400:
		info.Tier = "GREEN"
		info.DisplayRank = fmt.Sprintf("Green %d", rating)
	default:
		info.Tier = "GRAY"
		info.DisplayRank = fmt.Sprintf("Gray %d", rating)
	}

	return info
}

// TFTProfile represents TFT-specific profile info
type TFTProfile struct {
	GameName      string
	TagLine       string
	SummonerLevel int
	TFTRank       *TFTRankInfo
	DoubleUpRank  *TFTRankInfo
	HyperRollRank *TFTRankInfo
}

// GetTFTProfile gets a complete TFT profile for the current summoner
func (c *TFTClient) GetTFTProfile() (*TFTProfile, error) {
	summoner, err := c.lcu.GetCurrentSummoner()
	if err != nil {
		return nil, err
	}

	profile := &TFTProfile{
		GameName:      summoner.GameName,
		TagLine:       summoner.TagLine,
		SummonerLevel: summoner.SummonerLevel,
	}

	// Get all ranks (ignore errors for individual queues - they might not have played)
	if rank, err := c.GetTFTRank(); err == nil {
		profile.TFTRank = rank
	}
	if rank, err := c.GetDoubleUpRank(); err == nil {
		profile.DoubleUpRank = rank
	}
	if rank, err := c.GetHyperRollRating(); err == nil {
		profile.HyperRollRank = rank
	}

	return profile, nil
}
