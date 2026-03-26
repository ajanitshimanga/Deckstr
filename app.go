package main

import (
	"context"
	"fmt"
	"strings"

	"OpenSmurfManager/internal/accounts"
	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/riotapi"
	"OpenSmurfManager/internal/riotclient"
	"OpenSmurfManager/internal/storage"
	"OpenSmurfManager/internal/updater"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx      context.Context
	storage  *storage.StorageService
	accounts *accounts.AccountService
	updater  *updater.Updater
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize storage service
	storageService, err := storage.NewStorageService()
	if err != nil {
		println("Error initializing storage:", err.Error())
		return
	}
	a.storage = storageService
	a.accounts = accounts.NewAccountService(storageService)
	a.updater = updater.NewUpdater()
}

// ============================================
// Vault Management (exposed to frontend)
// ============================================

// VaultExists checks if a vault already exists
func (a *App) VaultExists() bool {
	return a.storage.VaultExists()
}

// IsUnlocked returns whether the vault is currently unlocked
func (a *App) IsUnlocked() bool {
	return a.storage.IsUnlocked()
}

// CreateVault creates a new vault with username and master password
func (a *App) CreateVault(username, masterPassword string) error {
	return a.storage.CreateVault(username, masterPassword)
}

// Unlock decrypts the vault with username and master password
func (a *App) Unlock(username, masterPassword string) error {
	return a.storage.Unlock(username, masterPassword)
}

// GetUsername returns the currently logged-in username
func (a *App) GetUsername() string {
	return a.storage.GetUsername()
}

// GetStoredUsername returns the username stored in vault (for pre-filling login)
func (a *App) GetStoredUsername() (string, error) {
	return a.storage.GetStoredUsername()
}

// Lock clears the vault from memory
func (a *App) Lock() {
	a.storage.Lock()
}

// GetVaultPath returns the path to the vault file
func (a *App) GetVaultPath() string {
	return a.storage.GetVaultPath()
}

// ============================================
// Account Management (exposed to frontend)
// ============================================

// GetAllAccounts returns all accounts
func (a *App) GetAllAccounts() ([]models.Account, error) {
	return a.accounts.GetAll()
}

// GetAccountByID returns a single account
func (a *App) GetAccountByID(id string) (*models.Account, error) {
	return a.accounts.GetByID(id)
}

// GetAccountsByNetwork returns accounts for a specific game network
func (a *App) GetAccountsByNetwork(networkID string) ([]models.Account, error) {
	return a.accounts.GetByNetwork(networkID)
}

// GetAccountsByTag returns accounts with a specific tag
func (a *App) GetAccountsByTag(tag string) ([]models.Account, error) {
	return a.accounts.GetByTag(tag)
}

// SearchAccounts searches accounts by query
func (a *App) SearchAccounts(query string) ([]models.Account, error) {
	return a.accounts.Search(query)
}

// CreateAccount creates a new account
func (a *App) CreateAccount(account models.Account) (*models.Account, error) {
	return a.accounts.Create(account)
}

// UpdateAccount updates an existing account
func (a *App) UpdateAccount(account models.Account) (*models.Account, error) {
	return a.accounts.Update(account)
}

// DeleteAccount removes an account
func (a *App) DeleteAccount(id string) error {
	return a.accounts.Delete(id)
}

// UpdateAccountRank updates rank info for an account
func (a *App) UpdateAccountRank(accountID, gameID, rank string, lp int) error {
	return a.accounts.UpdateRank(accountID, gameID, rank, lp)
}

// ============================================
// Tags & Game Networks (exposed to frontend)
// ============================================

// GetAllTags returns all available tags
func (a *App) GetAllTags() ([]string, error) {
	return a.accounts.GetAllTags()
}

// AddTag adds a new tag
func (a *App) AddTag(tag string) error {
	return a.accounts.AddTag(tag)
}

// GetGameNetworks returns all game networks
func (a *App) GetGameNetworks() ([]models.GameNetwork, error) {
	return a.accounts.GetGameNetworks()
}

// ============================================
// Settings (exposed to frontend)
// ============================================

// GetSettings returns the current settings
func (a *App) GetSettings() (*models.Settings, error) {
	data, err := a.storage.GetVaultData()
	if err != nil {
		return nil, err
	}
	return &data.Settings, nil
}

// UpdateSettings updates the settings
func (a *App) UpdateSettings(settings models.Settings) error {
	data, err := a.storage.GetVaultData()
	if err != nil {
		return err
	}
	data.Settings = settings
	if err := a.storage.UpdateVaultData(data); err != nil {
		return err
	}
	return a.storage.Save()
}

// ============================================
// Rank Detection & Auto-Update (exposed to frontend)
// ============================================

// DetectSignedInAccount detects the currently signed-in Riot account via LCU
// Returns the detected account info with ranks, or nil if no client is running
func (a *App) DetectSignedInAccount() (*riotclient.DetectedAccount, error) {
	return riotclient.DetectAndFetchRanks()
}

// MatchAndUpdateAccount matches detected account to stored accounts and updates ranks
// Returns the matched account ID if found, empty string otherwise
func (a *App) MatchAndUpdateAccount(detected *riotclient.DetectedAccount) (string, error) {
	if detected == nil {
		return "", nil
	}

	accounts, err := a.accounts.GetAll()
	if err != nil {
		return "", err
	}

	// Try to match by PUUID first (most reliable)
	var matched *models.Account
	if detected.PUUID != "" {
		matched = riotclient.MatchAccountByPUUID(accounts, detected.PUUID)
	}

	// Fall back to Riot ID matching
	if matched == nil && detected.RiotID != "" {
		matched = riotclient.MatchAccountByRiotID(accounts, detected.RiotID)
	}

	if matched == nil {
		return "", nil
	}

	// Update the account with detected ranks
	riotclient.UpdateAccountRanks(matched, detected)

	// Save the updated account
	_, err = a.accounts.Update(*matched)
	if err != nil {
		return "", err
	}

	return matched.ID, nil
}

// RefreshAccountRanksLCU refreshes ranks for an account using LCU (must be signed in)
func (a *App) RefreshAccountRanksLCU(accountID string) error {
	detected, err := riotclient.DetectAndFetchRanks()
	if err != nil {
		return err
	}

	account, err := a.accounts.GetByID(accountID)
	if err != nil {
		return err
	}

	// Verify the signed-in account matches
	riotIDLower := strings.ToLower(account.RiotID)
	detectedLower := strings.ToLower(detected.RiotID)
	if riotIDLower != detectedLower && account.PUUID != detected.PUUID {
		return fmt.Errorf("signed-in account (%s) does not match requested account (%s)", detected.RiotID, account.RiotID)
	}

	// Update ranks
	riotclient.UpdateAccountRanks(account, detected)
	_, err = a.accounts.Update(*account)
	return err
}

// RefreshAccountRanksAPI refreshes ranks using Riot API (BYOK)
func (a *App) RefreshAccountRanksAPI(accountID string) error {
	// Get API key from settings
	settings, err := a.GetSettings()
	if err != nil {
		return err
	}

	if settings.RiotAPIKey == "" {
		return fmt.Errorf("no Riot API key configured - add one in Settings")
	}

	account, err := a.accounts.GetByID(accountID)
	if err != nil {
		return err
	}

	if account.RiotID == "" {
		return fmt.Errorf("account has no Riot ID set")
	}

	// Determine platform/region
	platform := account.Region
	if platform == "" {
		platform = settings.DefaultRegion
	}
	if platform == "" {
		platform = riotapi.PlatformNA1 // Default to NA
	}

	// Determine which games to fetch
	games := account.Games
	if len(games) == 0 {
		games = []string{"lol", "tft"} // Default to both
	}

	// Fetch ranks via API
	client := riotapi.NewClient(settings.RiotAPIKey)
	ranks, err := client.FetchAllRanks(account.RiotID, platform, games)
	if err != nil {
		return err
	}

	// Update account
	account.CachedRanks = ranks
	_, err = a.accounts.Update(*account)
	return err
}

// GetRiotAPIStatus checks if Riot API key is configured
func (a *App) GetRiotAPIStatus() (bool, error) {
	settings, err := a.GetSettings()
	if err != nil {
		return false, err
	}
	return settings.RiotAPIKey != "", nil
}

// IsRiotClientRunning checks if any Riot client is running
func (a *App) IsRiotClientRunning() bool {
	_, err := riotclient.DetectAndFetchRanks()
	return err == nil
}

// ============================================
// Window Management (exposed to frontend)
// ============================================

// SetWindowSizeLogin sets the window to login/compact size (vertical)
func (a *App) SetWindowSizeLogin() {
	runtime.WindowSetMinSize(a.ctx, 380, 620)
	runtime.WindowSetSize(a.ctx, 380, 620)
}

// SetWindowSizeMain sets the window to main/dashboard size (horizontal)
func (a *App) SetWindowSizeMain() {
	runtime.WindowSetMinSize(a.ctx, 520, 760)
	runtime.WindowSetSize(a.ctx, 520, 760)
}

// ============================================
// Updates (exposed to frontend)
// ============================================

// GetAppVersion returns the current app version
func (a *App) GetAppVersion() string {
	return a.updater.GetCurrentVersion()
}

// CheckForUpdates checks GitHub for available updates
func (a *App) CheckForUpdates() (*updater.UpdateInfo, error) {
	return a.updater.CheckForUpdates()
}

// DownloadAndInstallUpdate downloads and installs the update
func (a *App) DownloadAndInstallUpdate(downloadURL string) error {
	// Download update (no progress channel for simplicity)
	installerPath, err := a.updater.DownloadUpdate(downloadURL, nil)
	if err != nil {
		return err
	}

	// Apply update (this will exit the app)
	return a.updater.ApplyUpdate(installerPath)
}

// OpenReleasePage opens the GitHub release page in browser
func (a *App) OpenReleasePage(url string) error {
	return a.updater.OpenReleasePage(url)
}
