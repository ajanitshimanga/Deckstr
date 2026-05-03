package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"OpenSmurfManager/internal/accounts"
	"OpenSmurfManager/internal/appdir"
	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/process"
	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/providers/epic"
	"OpenSmurfManager/internal/providers/riot"
	"OpenSmurfManager/internal/providers/steam"
	"OpenSmurfManager/internal/riotapi"
	"OpenSmurfManager/internal/storage"
	"OpenSmurfManager/internal/telemetry"
	"OpenSmurfManager/internal/updater"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx       context.Context
	storage   *storage.StorageService
	accounts  *accounts.AccountService
	updater   *updater.Updater
	providers *providers.Registry

	// startTime is set by main() before Wails.Run so startup() can report
	// cold-boot latency in the app.start telemetry event.
	startTime time.Time

	// quitting is flipped by the tray's Quit menu so that beforeClose
	// allows the next close to actually exit instead of hiding to tray.
	quitting atomic.Bool
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

	// Register all platform providers. Adding a new platform = adding one
	// MustRegister call here.
	a.providers = providers.NewRegistry()
	a.providers.MustRegister(riot.New())
	a.providers.MustRegister(epic.New())
	a.providers.MustRegister(steam.New())

	// App-start latency = time from main() to Wails runtime ready.
	// Separately emit has_vault so DAU/MAU queries can distinguish
	// brand-new installs from returning users.
	var latencyMs int64
	if !a.startTime.IsZero() {
		latencyMs = time.Since(a.startTime).Milliseconds()
	}
	telemetry.LogInfo("app.start", map[string]interface{}{
		"startup_latency_ms": latencyMs,
		"has_vault":          a.storage.VaultExists(),
	})
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
	err := a.storage.CreateVault(username, masterPassword)
	telemetry.LogInfo("vault.create", map[string]interface{}{
		"success":  err == nil,
		"variant":  "basic",
	})
	return err
}

// CreateVaultWithHint creates a new vault with username, master password, and optional hint
// DEPRECATED: Use CreateVaultWithRecoveryPhrase instead
func (a *App) CreateVaultWithHint(username, masterPassword, hint string) error {
	err := a.storage.CreateVaultWithHint(username, masterPassword, hint)
	telemetry.LogInfo("vault.create", map[string]interface{}{
		"success":  err == nil,
		"variant":  "with_hint",
		"has_hint": hint != "",
	})
	return err
}

// CreateVaultWithRecoveryPhrase creates a new vault and returns the recovery phrase
// The recovery phrase should be shown to the user (hidden by default) for safekeeping
func (a *App) CreateVaultWithRecoveryPhrase(username, masterPassword, hint string) (string, error) {
	phrase, err := a.storage.CreateVaultWithRecoveryPhrase(username, masterPassword, hint)
	telemetry.LogInfo("vault.create", map[string]interface{}{
		"success":  err == nil,
		"variant":  "with_recovery",
		"has_hint": hint != "",
	})
	return phrase, err
}

// Unlock decrypts the vault with username and master password
func (a *App) Unlock(username, masterPassword string) error {
	err := a.storage.Unlock(username, masterPassword)
	telemetry.LogInfo("vault.unlock", map[string]interface{}{"success": err == nil})
	return err
}

// GetPasswordHint returns the password hint (available without unlocking)
func (a *App) GetPasswordHint() (string, error) {
	return a.storage.GetPasswordHint()
}

// HasRecoveryPhrase checks if the vault has a recovery phrase set
func (a *App) HasRecoveryPhrase() (bool, error) {
	return a.storage.HasRecoveryPhrase()
}

// RotatedRecoveryPhrase is the frontend-facing shape for a recovery phrase
// that was just generated during the v1→v2 security migration. Present=true
// means the caller must display the phrase to the user and gate the app
// behind that display — the old phrase no longer works.
type RotatedRecoveryPhrase struct {
	Present bool   `json:"present"`
	Phrase  string `json:"phrase"`
}

// ConsumePendingRecoveryRotation drains the one-shot rotated-phrase slot
// and returns its contents. The cleartext is cleared from Go memory on this
// call so subsequent calls return Present=false. Frontend should call this
// exactly once immediately after a successful Unlock.
func (a *App) ConsumePendingRecoveryRotation() RotatedRecoveryPhrase {
	phrase, ok := a.storage.ConsumePendingRecoveryRotation()
	if !ok {
		return RotatedRecoveryPhrase{Present: false}
	}
	return RotatedRecoveryPhrase{Present: true, Phrase: phrase}
}

// GenerateRecoveryPhraseForLegacyUser generates a recovery phrase for existing users without one
// Must be called while vault is unlocked. Returns the new recovery phrase.
func (a *App) GenerateRecoveryPhraseForLegacyUser() (string, error) {
	return a.storage.GenerateRecoveryPhraseForLegacyUser()
}

// RegenerateRecoveryPhrase verifies password and generates a new recovery phrase
func (a *App) RegenerateRecoveryPhrase(password string) (string, error) {
	return a.storage.RegenerateRecoveryPhrase(password)
}

// ResetPasswordWithRecoveryPhrase resets the password using the 6-word recovery phrase
// Returns a NEW recovery phrase (old one is invalidated after use)
func (a *App) ResetPasswordWithRecoveryPhrase(recoveryPhrase, newPassword, newHint string) (string, error) {
	return a.storage.ResetPasswordWithRecoveryPhrase(recoveryPhrase, newPassword, newHint)
}

// ChangePassword changes the master password (must be unlocked, must provide correct current password)
// Returns a NEW recovery phrase (old one is invalidated)
func (a *App) ChangePassword(currentPassword, newPassword string) (string, error) {
	return a.storage.ChangePassword(currentPassword, newPassword)
}

// UpdatePasswordHint updates the password hint (must be unlocked)
// This allows legacy users to add/change their hint without changing password
func (a *App) UpdatePasswordHint(hint string) error {
	return a.storage.UpdatePasswordHint(hint)
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
	telemetry.LogInfo("vault.lock", nil)
}

// GetVaultPath returns the path to the vault file
func (a *App) GetVaultPath() string {
	return a.storage.GetVaultPath()
}

// DetectLegacyVault returns metadata about an orphaned vault.osm in the
// pre-rebrand OpenSmurfManager directory, or nil. Surfaced on the auth
// screens so users whose v1.5→1.6 migration left their real vault behind
// can adopt it without manually moving files.
func (a *App) DetectLegacyVault() (*storage.LegacyVaultInfo, error) {
	info, err := a.storage.DetectLegacyVault()
	telemetry.LogInfo("vault.legacy_detect", map[string]interface{}{
		"found":   info != nil,
		"success": err == nil,
	})
	return info, err
}

// AdoptLegacyVault replaces the current vault with the orphaned legacy one,
// archiving the existing vault to vault.osm.replaced-<unix> first. The
// vault must be locked when called. After success the caller should
// re-run the initialize() flow so the unlock screen pre-fills the
// adopted vault's username.
//
// Also carries client.id over (with the same archive-on-overwrite) and
// removes the entire legacy folder, so the user's telemetry identity
// stays continuous and the orphan probe stops reappearing.
func (a *App) AdoptLegacyVault() error {
	result, err := a.storage.AdoptLegacyVault()
	telemetry.LogInfo("vault.legacy_adopt", map[string]interface{}{
		"success": err == nil,
	})
	if err == nil {
		// Unified semantic event for "vault identity changed in a
		// non-recoverable way." Pairs with kind=import_file from
		// ImportVaultFromPath. Useful for tracking how often the rebrand
		// recovery actually fires + when we can sunset the legacy code.
		telemetry.LogInfo("vault.transition", map[string]interface{}{
			"kind":              "adopt_legacy",
			"archived_current":  result.ArchivedCurrent,
			"client_id_carried": result.ClientIDCarried,
			"legacy_dir_removed": result.LegacyDirRemoved,
		})
	}
	return err
}

// ImportVaultFromPath replaces the current vault with the bytes of the
// caller-supplied vault file. Same atomic replace-with-backup semantics as
// AdoptLegacyVault, but works with any source path — backup files, copies
// from another machine, vault.osm pulled off a dead disk. Refuses while
// unlocked.
func (a *App) ImportVaultFromPath(path string) error {
	err := a.storage.ImportVaultFromPath(path)
	telemetry.LogInfo("vault.import", map[string]interface{}{
		"success": err == nil,
	})
	if err == nil {
		telemetry.LogInfo("vault.transition", map[string]interface{}{
			"kind":             "import_file",
			"archived_current": true, // import always archives if a current existed; ImportVaultFromPath checks
			"client_id_carried": false,
			"legacy_dir_removed": false,
		})
	}
	return err
}

// OpenVaultFolder reveals the user-config directory containing vault.osm
// in the platform's native file manager. Used by the recovery UI as an
// escape hatch when the in-app banner fails for whatever reason — the
// user can at least see their files and copy them somewhere safe.
func (a *App) OpenVaultFolder() error {
	dir, err := appdir.Path()
	if err != nil {
		return err
	}
	return openInFileManager(dir)
}

// PickVaultFile opens a native file picker filtered to .osm files and
// returns the chosen absolute path, or "" if the user cancelled. Pure
// dialog passthrough — does not touch the vault. Pair with
// ImportVaultFromPath to actually swap.
func (a *App) PickVaultFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select vault file to import",
		Filters: []runtime.FileFilter{
			{DisplayName: "Vault files (*.osm)", Pattern: "*.osm"},
			{DisplayName: "All files (*.*)", Pattern: "*.*"},
		},
	})
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
	result, err := a.accounts.Create(account)
	telemetry.LogInfo("account.add", map[string]interface{}{
		"network_id":  account.NetworkID,
		"games_count": len(account.Games),
		"tags_count":  len(account.Tags),
		"success":     err == nil,
	})
	return result, err
}

// UpdateAccount updates an existing account
func (a *App) UpdateAccount(account models.Account) (*models.Account, error) {
	result, err := a.accounts.Update(account)
	telemetry.LogInfo("account.edit", map[string]interface{}{
		"network_id": account.NetworkID,
		"success":    err == nil,
	})
	return result, err
}

// DeleteAccount removes an account
func (a *App) DeleteAccount(id string) error {
	err := a.accounts.Delete(id)
	telemetry.LogInfo("account.delete", map[string]interface{}{"success": err == nil})
	return err
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

// DetectSignedInAccount detects the currently signed-in account from any
// registered platform provider. Returns the detected account info with ranks,
// or nil if no supported client is running.
func (a *App) DetectSignedInAccount() (*providers.DetectedAccount, error) {
	start := time.Now()
	detected, err := a.providers.DetectAny(a.ctx)
	result := "none"
	if err != nil {
		result = "error"
	} else if detected != nil {
		result = "detected"
	}
	telemetry.LogInfo("account.detect", map[string]interface{}{
		"result":       result,
		"duration_ms":  time.Since(start).Milliseconds(),
	})
	return detected, err
}

// IsInGame checks if the user is currently in an active game (not just client/lobby)
// Returns true if any game process from any supported game is running
// Used to pause polling during gameplay to avoid any performance impact
func (a *App) IsInGame() bool {
	// Collect all game processes for current platform from all supported games
	var allGameProcesses []string
	for _, network := range models.DefaultGameNetworks() {
		for _, game := range network.Games {
			allGameProcesses = append(allGameProcesses, game.GameProcesses.ForCurrentPlatform()...)
		}
	}

	return process.AnyRunning(allGameProcesses)
}

// GetActiveGameProcess returns the name of the currently running game process
// Returns empty string if no game is running (user is in lobby or client closed)
func (a *App) GetActiveGameProcess() string {
	var allGameProcesses []string
	for _, network := range models.DefaultGameNetworks() {
		for _, game := range network.Games {
			allGameProcesses = append(allGameProcesses, game.GameProcesses.ForCurrentPlatform()...)
		}
	}

	return process.GetRunningProcess(allGameProcesses)
}

// MatchAndUpdateAccount matches a detected account to stored accounts and
// updates ranks via the owning provider. Returns the matched account ID if
// found, empty string otherwise.
func (a *App) MatchAndUpdateAccount(detected *providers.DetectedAccount) (string, error) {
	if detected == nil {
		return "", nil
	}

	accounts, err := a.accounts.GetAll()
	if err != nil {
		return "", err
	}

	matched := a.providers.MatchAccount(accounts, detected)
	if matched == nil {
		return "", nil
	}

	a.providers.UpdateAccount(matched, detected)

	if _, err := a.accounts.Update(*matched); err != nil {
		return "", err
	}

	return matched.ID, nil
}

// RefreshAccountRanksLCU refreshes ranks for a stored account from the
// currently signed-in client session. The session must belong to the same
// account being refreshed.
func (a *App) RefreshAccountRanksLCU(accountID string) error {
	detected, err := a.providers.DetectAny(a.ctx)
	if err != nil {
		return err
	}
	if detected == nil {
		return fmt.Errorf("no signed-in client detected")
	}

	account, err := a.accounts.GetByID(accountID)
	if err != nil {
		return err
	}

	if account.NetworkID != "" && account.NetworkID != detected.NetworkID {
		return fmt.Errorf("signed-in %s account does not match requested %s account",
			detected.NetworkID, account.NetworkID)
	}

	// Verify the signed-in account matches the one being refreshed.
	matchedAccount := a.providers.MatchAccount([]models.Account{*account}, detected)
	if matchedAccount == nil {
		return fmt.Errorf("signed-in account (%s) does not match requested account (%s)",
			detected.DisplayName, strings.TrimSpace(account.DisplayName))
	}

	a.providers.UpdateAccount(account, detected)
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

// IsRiotClientRunning checks if the Riot client is running. Kept for
// backwards compatibility with the frontend - prefer IsAnyClientRunning.
func (a *App) IsRiotClientRunning() bool {
	p := a.providers.Get(riot.NetworkID)
	if p == nil {
		return false
	}
	return p.IsClientRunning(a.ctx)
}

// IsAnyClientRunning returns true if any registered platform's client is
// currently running. Used by polling logic to decide whether to attempt
// detection.
func (a *App) IsAnyClientRunning() bool {
	return a.providers.IsAnyClientRunning(a.ctx)
}

// ============================================
// Window Management (exposed to frontend)
// ============================================

// SetWindowSizeLogin sets the window to login size. Same dimensions as the
// main dashboard so unlocking doesn't trigger a jarring resize.
func (a *App) SetWindowSizeLogin() {
	runtime.WindowSetMinSize(a.ctx, 520, 760)
	runtime.WindowSetSize(a.ctx, 520, 760)
}

// SetWindowSizeMain sets the window to main/dashboard size.
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

// ============================================
// Telemetry bridge (exposed to frontend)
// ============================================

// LogEvent records a UI-layer telemetry event. Level must be one of
// "info", "warn", "error" (anything else is treated as info). Attributes
// are stringly-typed at this boundary for simple Wails bindings; the
// Go-side logger widens them into the OTel attribute shape.
//
// Callers must never pass credentials, usernames, or other vault data.
// The frontend wrapper in lib/telemetry.ts holds the whitelist.
func (a *App) LogEvent(level, event string, attributes map[string]string) {
	attrs := make(map[string]interface{}, len(attributes)+1)
	for k, v := range attributes {
		attrs[k] = v
	}
	attrs["source"] = "frontend"

	switch strings.ToLower(level) {
	case "error":
		telemetry.LogError(event, attrs)
	case "warn":
		telemetry.Log(telemetry.SeverityWarn, event, attrs)
	default:
		telemetry.LogInfo(event, attrs)
	}
}

// IsTelemetryEnabled reports whether usage analytics are currently active.
// Backed by the opt-out marker file written at install time; also mutated
// by SetTelemetryEnabled below.
func (a *App) IsTelemetryEnabled() bool {
	return !telemetry.IsDisabled()
}

// SetTelemetryEnabled toggles usage analytics on/off and applies the
// change immediately: on disable, the active logger is closed so no new
// events are written; on enable, a fresh logger is started. Survives app
// restarts via the marker file.
func (a *App) SetTelemetryEnabled(enabled bool) error {
	if err := telemetry.SetDisabled(!enabled); err != nil {
		return err
	}
	if enabled {
		return telemetry.Init("Deckstr", updater.Version)
	}
	return telemetry.Close()
}

// OpenUsageLogsFolder reveals the rotated log directory in the user's
// file manager so they can inspect or export the raw JSON-line records.
// Promised by TELEMETRY.md as a transparency control.
func (a *App) OpenUsageLogsFolder() error {
	path, err := telemetry.LogsPath()
	if err != nil {
		return err
	}
	return openInFileManager(path)
}
