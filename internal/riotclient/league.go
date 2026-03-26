package riotclient

import (
	"fmt"
)

// LeagueClient handles League of Legends specific API calls
type LeagueClient struct {
	lcu *LCUClient
}

// NewLeagueClient creates a new League client
func NewLeagueClient(lcu *LCUClient) *LeagueClient {
	return &LeagueClient{lcu: lcu}
}

// RankedStats represents ranked statistics for a queue
type RankedStats struct {
	Division                       string   `json:"division"`
	HighestDivision                string   `json:"highestDivision"`
	HighestTier                    string   `json:"highestTier"`
	IsProvisional                  bool     `json:"isProvisional"`
	LeaguePoints                   int      `json:"leaguePoints"`
	Losses                         int      `json:"losses"`
	MiniSeriesProgress             string   `json:"miniSeriesProgress"`
	PreviousSeasonAchievedDivision string   `json:"previousSeasonAchievedDivision"`
	PreviousSeasonAchievedTier     string   `json:"previousSeasonAchievedTier"`
	PreviousSeasonEndDivision      string   `json:"previousSeasonEndDivision"`
	PreviousSeasonEndTier          string   `json:"previousSeasonEndTier"`
	ProvisionalGameThreshold       int      `json:"provisionalGameThreshold"`
	ProvisionalGamesRemaining      int      `json:"provisionalGamesRemaining"`
	QueueType                      string   `json:"queueType"`
	RatedRating                    int      `json:"ratedRating"`
	RatedTier                      string   `json:"ratedTier"`
	Tier                           string   `json:"tier"`
	Warnings                       *Warning `json:"warnings"`
	Wins                           int      `json:"wins"`
}

// Warning represents rank decay warnings
type Warning struct {
	DaysUntilDecay       int  `json:"daysUntilDecay"`
	DemotionWarning      int  `json:"demotionWarning"`
	DisplayDecayWarning  bool `json:"displayDecayWarning"`
	GamesRemaining       int  `json:"gamesRemaining"`
	TimeUntilInactivity  int  `json:"timeUntilInactivity"`
}

// RankedData represents all ranked data for a player
type RankedData struct {
	EarnedRegaliaRewardIds []string               `json:"earnedRegaliaRewardIds"`
	HighestCurrentSeasonReachedTierSR string     `json:"highestCurrentSeasonReachedTierSR"`
	HighestPreviousSeasonAchievedTier string     `json:"highestPreviousSeasonAchievedTier"`
	HighestPreviousSeasonEndTier      string     `json:"highestPreviousSeasonEndTier"`
	HighestRankedEntry                *RankedStats `json:"highestRankedEntry"`
	HighestRankedEntrySR              *RankedStats `json:"highestRankedEntrySR"`
	QueueMap                          map[string]RankedStats `json:"queueMap"`
	Queues                            []RankedStats `json:"queues"`
	RankedRegaliaLevel                int          `json:"rankedRegaliaLevel"`
	Seasons                           map[string]interface{} `json:"seasons"`
	SplitsProgress                    map[string]interface{} `json:"splitsProgress"`
}

// LeagueRankInfo contains formatted rank information for League
type LeagueRankInfo struct {
	QueueType    string // RANKED_SOLO_5x5, RANKED_FLEX_SR, etc.
	Tier         string // IRON, BRONZE, SILVER, GOLD, PLATINUM, EMERALD, DIAMOND, MASTER, GRANDMASTER, CHALLENGER
	Division     string // I, II, III, IV
	LP           int
	Wins         int
	Losses       int
	Provisional  bool
	DisplayRank  string // e.g., "Diamond II 45 LP"
}

// GetRankedStats gets the ranked stats for the current summoner
func (c *LeagueClient) GetRankedStats() (*RankedData, error) {
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

// GetSoloQueueRank gets the solo queue rank for the current summoner
func (c *LeagueClient) GetSoloQueueRank() (*LeagueRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	if stats, ok := ranked.QueueMap["RANKED_SOLO_5x5"]; ok {
		return formatRankInfo("RANKED_SOLO_5x5", &stats), nil
	}

	return nil, fmt.Errorf("no solo queue rank found")
}

// GetFlexRank gets the flex queue rank for the current summoner
func (c *LeagueClient) GetFlexRank() (*LeagueRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	if stats, ok := ranked.QueueMap["RANKED_FLEX_SR"]; ok {
		return formatRankInfo("RANKED_FLEX_SR", &stats), nil
	}

	return nil, fmt.Errorf("no flex queue rank found")
}

// GetAllRanks gets all ranked queue stats
func (c *LeagueClient) GetAllRanks() (map[string]*LeagueRankInfo, error) {
	ranked, err := c.GetRankedStats()
	if err != nil {
		return nil, err
	}

	ranks := make(map[string]*LeagueRankInfo)
	for queueType, stats := range ranked.QueueMap {
		// Only include queues with actual ranks
		if stats.Tier != "" && stats.Tier != "NONE" {
			ranks[queueType] = formatRankInfo(queueType, &stats)
		}
	}

	return ranks, nil
}

func formatRankInfo(queueType string, stats *RankedStats) *LeagueRankInfo {
	info := &LeagueRankInfo{
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

	return info
}

// MatchHistory represents recent match data
type MatchHistoryGame struct {
	GameID       int64  `json:"gameId"`
	GameMode     string `json:"gameMode"`
	GameType     string `json:"gameType"`
	QueueID      int    `json:"queueId"`
	GameCreation int64  `json:"gameCreation"`
	Win          bool   `json:"win"`
	ChampionID   int    `json:"championId"`
	ChampionName string `json:"championName"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
}

// GetRecentMatches gets recent match history
func (c *LeagueClient) GetRecentMatches(count int) ([]MatchHistoryGame, error) {
	// This endpoint might vary based on the client version
	endpoint := fmt.Sprintf("/lol-match-history/v1/products/lol/current-summoner/matches?begIndex=0&endIndex=%d", count)

	var response struct {
		Games struct {
			Games []MatchHistoryGame `json:"games"`
		} `json:"games"`
	}

	err := c.lcu.GetJSON(endpoint, &response)
	if err != nil {
		return nil, err
	}

	return response.Games.Games, nil
}

// SummonerProfile represents a summoner's profile info
type SummonerProfile struct {
	GameName      string `json:"gameName"`
	TagLine       string `json:"tagLine"`
	SummonerLevel int    `json:"summonerLevel"`
	ProfileIconID int    `json:"profileIconId"`
	PUUID         string `json:"puuid"`
}

// GetCurrentProfile gets the current summoner's profile
func (c *LeagueClient) GetCurrentProfile() (*SummonerProfile, error) {
	summoner, err := c.lcu.GetCurrentSummoner()
	if err != nil {
		return nil, err
	}

	return &SummonerProfile{
		GameName:      summoner.GameName,
		TagLine:       summoner.TagLine,
		SummonerLevel: summoner.SummonerLevel,
		ProfileIconID: summoner.ProfileIconID,
		PUUID:         summoner.PUUID,
	}, nil
}

// LCUChampionMastery represents champion mastery from LCU
type LCUChampionMastery struct {
	ChampionID                   int   `json:"championId"`
	ChampionLevel                int   `json:"championLevel"`
	ChampionPoints               int   `json:"championPoints"`
	ChampionPointsSinceLastLevel int   `json:"championPointsSinceLastLevel"`
	ChampionPointsUntilNextLevel int   `json:"championPointsUntilNextLevel"`
	LastPlayTime                 int64 `json:"lastPlayTime"`
	MarkRequiredForNextLevel     int   `json:"markRequiredForNextLevel"`
	TokensEarned                 int   `json:"tokensEarned"`
}

// GetTopChampionMasteries gets the top N champion masteries for the current summoner
func (c *LeagueClient) GetTopChampionMasteries(count int) ([]LCUChampionMastery, error) {
	summoner, err := c.lcu.GetCurrentSummoner()
	if err != nil {
		return nil, fmt.Errorf("failed to get current summoner: %w", err)
	}

	endpoint := fmt.Sprintf("/lol-collections/v1/inventories/%s/champion-mastery/top?limit=%d", summoner.PUUID, count)

	var masteries []LCUChampionMastery
	err = c.lcu.GetJSON(endpoint, &masteries)
	if err != nil {
		// Try alternate endpoint
		endpoint = fmt.Sprintf("/lol-champion-mastery/v1/local-player/champion-mastery/top?count=%d", count)
		err = c.lcu.GetJSON(endpoint, &masteries)
		if err != nil {
			return nil, fmt.Errorf("failed to get champion masteries: %w", err)
		}
	}

	return masteries, nil
}

// Champion ID to Name mapping (common champions - can be expanded or fetched from Data Dragon)
var championNames = map[int]string{
	1: "Annie", 2: "Olaf", 3: "Galio", 4: "Twisted Fate", 5: "Xin Zhao",
	6: "Urgot", 7: "LeBlanc", 8: "Vladimir", 9: "Fiddlesticks", 10: "Kayle",
	11: "Master Yi", 12: "Alistar", 13: "Ryze", 14: "Sion", 15: "Sivir",
	16: "Soraka", 17: "Teemo", 18: "Tristana", 19: "Warwick", 20: "Nunu & Willump",
	21: "Miss Fortune", 22: "Ashe", 23: "Tryndamere", 24: "Jax", 25: "Morgana",
	26: "Zilean", 27: "Singed", 28: "Evelynn", 29: "Twitch", 30: "Karthus",
	31: "Cho'Gath", 32: "Amumu", 33: "Rammus", 34: "Anivia", 35: "Shaco",
	36: "Dr. Mundo", 37: "Sona", 38: "Kassadin", 39: "Irelia", 40: "Janna",
	41: "Gangplank", 42: "Corki", 43: "Karma", 44: "Taric", 45: "Veigar",
	48: "Trundle", 50: "Swain", 51: "Caitlyn", 53: "Blitzcrank", 54: "Malphite",
	55: "Katarina", 56: "Nocturne", 57: "Maokai", 58: "Renekton", 59: "Jarvan IV",
	60: "Elise", 61: "Orianna", 62: "Wukong", 63: "Brand", 64: "Lee Sin",
	67: "Vayne", 68: "Rumble", 69: "Cassiopeia", 72: "Skarner", 74: "Heimerdinger",
	75: "Nasus", 76: "Nidalee", 77: "Udyr", 78: "Poppy", 79: "Gragas",
	80: "Pantheon", 81: "Ezreal", 82: "Mordekaiser", 83: "Yorick", 84: "Akali",
	85: "Kennen", 86: "Garen", 89: "Leona", 90: "Malzahar", 91: "Talon",
	92: "Riven", 96: "Kog'Maw", 98: "Shen", 99: "Lux", 101: "Xerath",
	102: "Shyvana", 103: "Ahri", 104: "Graves", 105: "Fizz", 106: "Volibear",
	107: "Rengar", 110: "Varus", 111: "Nautilus", 112: "Viktor", 113: "Sejuani",
	114: "Fiora", 115: "Ziggs", 117: "Lulu", 119: "Draven", 120: "Hecarim",
	121: "Kha'Zix", 122: "Darius", 126: "Jayce", 127: "Lissandra", 131: "Diana",
	133: "Quinn", 134: "Syndra", 136: "Aurelion Sol", 141: "Kayn", 142: "Zoe",
	143: "Zyra", 145: "Kai'Sa", 147: "Seraphine", 150: "Gnar", 154: "Zac",
	157: "Yasuo", 161: "Vel'Koz", 163: "Taliyah", 166: "Akshan", 164: "Camille",
	200: "Bel'Veth", 201: "Braum", 202: "Jhin", 203: "Kindred", 221: "Zeri",
	222: "Jinx", 223: "Tahm Kench", 233: "Briar", 234: "Viego", 235: "Senna",
	236: "Lucian", 238: "Zed", 240: "Kled", 245: "Ekko", 246: "Qiyana",
	254: "Vi", 266: "Aatrox", 267: "Nami", 268: "Azir", 350: "Yuumi",
	360: "Samira", 412: "Thresh", 420: "Illaoi", 421: "Rek'Sai", 427: "Ivern",
	429: "Kalista", 432: "Bard", 497: "Rakan", 498: "Xayah", 516: "Ornn",
	517: "Sylas", 518: "Neeko", 523: "Aphelios", 526: "Rell", 555: "Pyke",
	711: "Vex", 777: "Yone", 799: "Ambessa", 875: "Sett", 876: "Lillia",
	887: "Gwen", 888: "Renata Glasc", 893: "Aurora", 895: "Nilah", 897: "K'Sante",
	901: "Smolder", 902: "Milio", 910: "Hwei", 950: "Naafiri",
}

// GetChampionName returns the champion name for an ID
func GetChampionName(championID int) string {
	if name, ok := championNames[championID]; ok {
		return name
	}
	return fmt.Sprintf("Champion %d", championID)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	// Convert DIAMOND -> Diamond
	lower := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if i == 0 {
			lower[i] = s[i] // Keep first char uppercase
		} else {
			if s[i] >= 'A' && s[i] <= 'Z' {
				lower[i] = s[i] + 32 // Convert to lowercase
			} else {
				lower[i] = s[i]
			}
		}
	}
	return string(lower)
}
