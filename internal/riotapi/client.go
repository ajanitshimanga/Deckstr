package riotapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"OpenSmurfManager/internal/models"
)

// Client handles Riot API requests using a user-provided API key (BYOK)
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Riot API client with the provided API key
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Regional routing values for account-v1
const (
	RegionAmericas = "americas"
	RegionEurope   = "europe"
	RegionAsia     = "asia"
)

// Platform routing values for game-specific APIs
const (
	PlatformNA1  = "na1"
	PlatformEUW1 = "euw1"
	PlatformEUN1 = "eun1"
	PlatformKR   = "kr"
	PlatformBR1  = "br1"
	PlatformJP1  = "jp1"
	PlatformOC1  = "oc1"
	PlatformLA1  = "la1"
	PlatformLA2  = "la2"
	PlatformTR1  = "tr1"
	PlatformRU   = "ru"
)

// GetRegionForPlatform returns the regional routing value for a platform
func GetRegionForPlatform(platform string) string {
	switch platform {
	case PlatformNA1, PlatformBR1, PlatformLA1, PlatformLA2, PlatformOC1:
		return RegionAmericas
	case PlatformEUW1, PlatformEUN1, PlatformTR1, PlatformRU:
		return RegionEurope
	case PlatformKR, PlatformJP1:
		return RegionAsia
	default:
		return RegionAmericas
	}
}

// Account represents a Riot account from the API
type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

// Summoner represents a League/TFT summoner from the API
type Summoner struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	PUUID         string `json:"puuid"`
	ProfileIconID int    `json:"profileIconId"`
	SummonerLevel int64  `json:"summonerLevel"`
}

// LeagueEntry represents ranked data from the API
type LeagueEntry struct {
	LeagueID     string `json:"leagueId"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	SummonerID   string `json:"summonerId"`
	SummonerName string `json:"summonerName"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	Veteran      bool   `json:"veteran"`
	Inactive     bool   `json:"inactive"`
	FreshBlood   bool   `json:"freshBlood"`
	HotStreak    bool   `json:"hotStreak"`
}

// GetAccountByRiotID looks up an account by Riot ID (gameName#tagLine)
func (c *Client) GetAccountByRiotID(gameName, tagLine, region string) (*Account, error) {
	// URL encode the game name (can have spaces and special chars)
	encodedName := url.PathEscape(gameName)
	encodedTag := url.PathEscape(tagLine)

	url := fmt.Sprintf("https://%s.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s",
		region, encodedName, encodedTag)

	var account Account
	if err := c.get(url, &account); err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

// GetSummonerByPUUID gets summoner info from PUUID
func (c *Client) GetSummonerByPUUID(puuid, platform string) (*Summoner, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/lol/summoner/v4/summoners/by-puuid/%s",
		platform, puuid)

	var summoner Summoner
	if err := c.get(url, &summoner); err != nil {
		return nil, fmt.Errorf("failed to get summoner: %w", err)
	}

	return &summoner, nil
}

// GetLeagueEntries gets ranked entries for a summoner
func (c *Client) GetLeagueEntries(summonerID, platform string) ([]LeagueEntry, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/lol/league/v4/entries/by-summoner/%s",
		platform, summonerID)

	var entries []LeagueEntry
	if err := c.get(url, &entries); err != nil {
		return nil, fmt.Errorf("failed to get league entries: %w", err)
	}

	return entries, nil
}

// GetTFTLeagueEntries gets TFT ranked entries for a summoner
func (c *Client) GetTFTLeagueEntries(summonerID, platform string) ([]LeagueEntry, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/tft/league/v1/entries/by-summoner/%s",
		platform, summonerID)

	var entries []LeagueEntry
	if err := c.get(url, &entries); err != nil {
		return nil, fmt.Errorf("failed to get TFT league entries: %w", err)
	}

	return entries, nil
}

// FetchAllRanks fetches all ranks for an account and returns cached rank data
func (c *Client) FetchAllRanks(riotID, platform string, games []string) ([]models.CachedRank, error) {
	gameName, tagLine, ok := models.ParseRiotID(riotID)
	if !ok {
		return nil, fmt.Errorf("invalid Riot ID format: %s", riotID)
	}

	region := GetRegionForPlatform(platform)

	// Step 1: Get PUUID from Riot ID
	account, err := c.GetAccountByRiotID(gameName, tagLine, region)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup account: %w", err)
	}

	// Step 2: Get Summoner ID from PUUID
	summoner, err := c.GetSummonerByPUUID(account.PUUID, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get summoner: %w", err)
	}

	var ranks []models.CachedRank
	now := time.Now()

	// Step 3: Get League ranks if requested
	if containsGame(games, "lol") {
		entries, err := c.GetLeagueEntries(summoner.ID, platform)
		if err == nil {
			for _, entry := range entries {
				ranks = append(ranks, models.CachedRank{
					GameID:      "lol",
					QueueType:   entry.QueueType,
					QueueName:   getQueueName(entry.QueueType),
					Tier:        entry.Tier,
					Division:    entry.Rank,
					LP:          entry.LeaguePoints,
					Wins:        entry.Wins,
					Losses:      entry.Losses,
					DisplayRank: formatDisplayRank(entry.Tier, entry.Rank, entry.LeaguePoints),
					LastUpdated: now,
				})
			}
		}
	}

	// Step 4: Get TFT ranks if requested
	if containsGame(games, "tft") {
		entries, err := c.GetTFTLeagueEntries(summoner.ID, platform)
		if err == nil {
			for _, entry := range entries {
				ranks = append(ranks, models.CachedRank{
					GameID:      "tft",
					QueueType:   entry.QueueType,
					QueueName:   getQueueName(entry.QueueType),
					Tier:        entry.Tier,
					Division:    entry.Rank,
					LP:          entry.LeaguePoints,
					Wins:        entry.Wins,
					Losses:      entry.Losses,
					DisplayRank: formatDisplayRank(entry.Tier, entry.Rank, entry.LeaguePoints),
					LastUpdated: now,
				})
			}
		}
	}

	return ranks, nil
}

func (c *Client) get(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Riot-Token", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == 404 {
		return fmt.Errorf("not found")
	}

	if resp.StatusCode == 403 {
		return fmt.Errorf("forbidden - check API key")
	}

	if resp.StatusCode == 429 {
		return fmt.Errorf("rate limited - try again later")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return json.Unmarshal(body, target)
}

func containsGame(games []string, game string) bool {
	for _, g := range games {
		if g == game {
			return true
		}
	}
	return false
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

func formatDisplayRank(tier, division string, lp int) string {
	if tier == "" {
		return "Unranked"
	}

	tierFormatted := capitalizeFirst(tier)

	// Apex tiers don't have divisions
	if tier == "MASTER" || tier == "GRANDMASTER" || tier == "CHALLENGER" {
		return fmt.Sprintf("%s %d LP", tierFormatted, lp)
	}

	return fmt.Sprintf("%s %s %d LP", tierFormatted, division, lp)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if i == 0 {
			result[i] = s[i]
		} else if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + 32
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}
