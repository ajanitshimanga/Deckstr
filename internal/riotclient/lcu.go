package riotclient

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LCUClient handles communication with the League Client Update API
type LCUClient struct {
	port         string
	authToken    string
	httpClient   *http.Client
	connected    bool
	lockfilePath string // Store path for validation
}

// LockfileData contains parsed lockfile information
type LockfileData struct {
	ProcessName string
	PID         string
	Port        string
	Password    string
	Protocol    string
	Path        string // Path to the lockfile for validation
}

// NewLCUClient creates a new LCU client by reading the lockfile
func NewLCUClient() (*LCUClient, error) {
	lockfile, err := findAndParseLockfileWithPath("riot")
	if err != nil {
		return nil, err
	}

	client := &LCUClient{
		port:         lockfile.Port,
		authToken:    base64.StdEncoding.EncodeToString([]byte("riot:" + lockfile.Password)),
		lockfilePath: lockfile.Path,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // Fast timeout - client should respond quickly or fail
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // LCU uses self-signed cert
				},
			},
		},
		connected: true,
	}

	return client, nil
}

// FindAndParseLockfile locates and parses the Riot lockfile (prefers Riot Client)
func FindAndParseLockfile() (*LockfileData, error) {
	return findLockfileByType("riot")
}

// FindLeagueLockfile finds specifically the League of Legends lockfile
func FindLeagueLockfile() (*LockfileData, error) {
	return findLockfileByType("league")
}

func findLockfileByType(clientType string) (*LockfileData, error) {
	homeDir, _ := os.UserHomeDir()
	localAppData := filepath.Join(homeDir, "AppData", "Local")

	var paths []string

	if clientType == "league" {
		// League-specific paths first
		paths = []string{
			filepath.Join(localAppData, "Riot Games", "League of Legends", "lockfile"),
			`C:\Riot Games\League of Legends\lockfile`,
			`D:\Riot Games\League of Legends\lockfile`,
		}
	} else {
		// Riot Client paths first
		paths = []string{
			filepath.Join(localAppData, "Riot Games", "Riot Client", "Config", "lockfile"),
			filepath.Join(localAppData, "Riot Games", "League of Legends", "lockfile"),
			`C:\Riot Games\League of Legends\lockfile`,
			`C:\Riot Games\Riot Client\Config\lockfile`,
			`D:\Riot Games\League of Legends\lockfile`,
			`D:\Riot Games\Riot Client\Config\lockfile`,
		}
	}

	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	if programFiles != "" {
		paths = append(paths, filepath.Join(programFiles, "Riot Games", "League of Legends", "lockfile"))
	}
	if programFilesX86 != "" {
		paths = append(paths, filepath.Join(programFilesX86, "Riot Games", "League of Legends", "lockfile"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return parseLockfile(p)
		}
	}

	if clientType == "league" {
		return nil, fmt.Errorf("League of Legends lockfile not found - is League running?")
	}
	return nil, fmt.Errorf("lockfile not found - is the Riot Client running?")
}

// NewLeagueLCUClient creates a client specifically for the League of Legends client
func NewLeagueLCUClient() (*LCUClient, error) {
	lockfile, err := findAndParseLockfileWithPath("league")
	if err != nil {
		return nil, err
	}

	client := &LCUClient{
		port:         lockfile.Port,
		authToken:    base64.StdEncoding.EncodeToString([]byte("riot:" + lockfile.Password)),
		lockfilePath: lockfile.Path,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // Fast timeout - client should respond quickly or fail
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		connected: true,
	}

	return client, nil
}

func parseLockfile(path string) (*LockfileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	parts := strings.Split(string(data), ":")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid lockfile format")
	}

	return &LockfileData{
		ProcessName: parts[0],
		PID:         parts[1],
		Port:        parts[2],
		Password:    parts[3],
		Protocol:    parts[4],
		Path:        path,
	}, nil
}

// findAndParseLockfileWithPath finds and parses lockfile, returning the path
func findAndParseLockfileWithPath(clientType string) (*LockfileData, error) {
	return findLockfileByType(clientType)
}

// IsConnected returns whether the client is connected
func (c *LCUClient) IsConnected() bool {
	return c.connected
}

// IsValid checks if the lockfile still exists (client still running)
func (c *LCUClient) IsValid() bool {
	if c.lockfilePath == "" {
		return c.connected
	}
	if _, err := os.Stat(c.lockfilePath); os.IsNotExist(err) {
		c.connected = false
		return false
	}
	return c.connected
}

// Get performs a GET request to the LCU API
func (c *LCUClient) Get(endpoint string) ([]byte, error) {
	url := fmt.Sprintf("https://127.0.0.1:%s%s", c.port, endpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Basic "+c.authToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LCU request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetJSON performs a GET request and unmarshals the response into the target
func (c *LCUClient) GetJSON(endpoint string, target interface{}) error {
	data, err := c.Get(endpoint)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// CurrentSummoner represents the logged-in summoner info
type CurrentSummoner struct {
	AccountID                   int64  `json:"accountId"`
	DisplayName                 string `json:"displayName"`
	GameName                    string `json:"gameName"`
	TagLine                     string `json:"tagLine"`
	InternalName                string `json:"internalName"`
	PercentCompleteForNextLevel int    `json:"percentCompleteForNextLevel"`
	ProfileIconID               int    `json:"profileIconId"`
	PUUID                       string `json:"puuid"`
	RerollPoints                struct {
		CurrentPoints    int `json:"currentPoints"`
		MaxRolls         int `json:"maxRolls"`
		NumberOfRolls    int `json:"numberOfRolls"`
		PointsCostToRoll int `json:"pointsCostToRoll"`
		PointsToReroll   int `json:"pointsToReroll"`
	} `json:"rerollPoints"`
	SummonerID    int64 `json:"summonerId"`
	SummonerLevel int   `json:"summonerLevel"`
	Unnamed       bool  `json:"unnamed"`
	XpSinceLastLevel int `json:"xpSinceLastLevel"`
	XpUntilNextLevel int `json:"xpUntilNextLevel"`
}

// GetCurrentSummoner gets the currently logged in summoner
func (c *LCUClient) GetCurrentSummoner() (*CurrentSummoner, error) {
	var summoner CurrentSummoner
	err := c.GetJSON("/lol-summoner/v1/current-summoner", &summoner)
	if err != nil {
		return nil, err
	}
	return &summoner, nil
}

// RiotUserInfo represents user info from the Riot auth system
type RiotUserInfo struct {
	Country string `json:"country"`
	Sub     string `json:"sub"` // PUUID
	Locale  string `json:"lol_locale"`
	// Riot ID info
	Acct struct {
		GameName string `json:"game_name"`
		TagLine  string `json:"tag_line"`
	} `json:"acct"`
}

// GetRiotUserInfo gets the Riot account user info from Riot Client
func (c *LCUClient) GetRiotUserInfo() (*RiotUserInfo, error) {
	// Get raw userinfo string first
	data, err := c.Get("/rso-auth/v1/authorization/userinfo")
	if err != nil {
		return nil, err
	}

	// The response is a JSON string containing JSON, so we need to unmarshal twice
	var jsonStr string
	if err := json.Unmarshal(data, &jsonStr); err != nil {
		// Try direct unmarshal if it's not a string
		var userInfo RiotUserInfo
		if err := json.Unmarshal(data, &userInfo); err != nil {
			return nil, fmt.Errorf("failed to parse userinfo: %w", err)
		}
		return &userInfo, nil
	}

	var userInfo RiotUserInfo
	if err := json.Unmarshal([]byte(jsonStr), &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse inner userinfo: %w", err)
	}
	return &userInfo, nil
}

// RiotClientUserInfo represents the riot-client-auth userinfo
type RiotClientUserInfo struct {
	PUUID     string `json:"sub"`
	GameName  string // Will be extracted from acct
	TagLine   string // Will be extracted from acct
}

// GetRiotClientAuth gets auth info from riot-client-auth endpoint
func (c *LCUClient) GetRiotClientAuth() (*RiotClientUserInfo, error) {
	data, err := c.Get("/riot-client-auth/v1/userinfo")
	if err != nil {
		return nil, err
	}

	// Parse the response - it may be a JSON string containing JSON
	var rawResponse interface{}
	if err := json.Unmarshal(data, &rawResponse); err != nil {
		return nil, err
	}

	var userInfoStr string
	switch v := rawResponse.(type) {
	case string:
		userInfoStr = v
	default:
		// Already a JSON object
		userInfoStr = string(data)
	}

	// Parse the actual user info
	var parsed struct {
		Sub  string `json:"sub"`
		Acct struct {
			GameName string `json:"game_name"`
			TagLine  string `json:"tag_line"`
		} `json:"acct"`
	}

	if err := json.Unmarshal([]byte(userInfoStr), &parsed); err != nil {
		return nil, err
	}

	return &RiotClientUserInfo{
		PUUID:    parsed.Sub,
		GameName: parsed.Acct.GameName,
		TagLine:  parsed.Acct.TagLine,
	}, nil
}

// ProductSession represents an active game session
type ProductSession struct {
	ProductID string `json:"productId"`
}

// GetProductSessions gets active product sessions (which games are running)
func (c *LCUClient) GetProductSessions() (map[string]ProductSession, error) {
	var sessions map[string]ProductSession
	err := c.GetJSON("/product-session/v1/external-sessions", &sessions)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}
