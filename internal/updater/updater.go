package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// These are set at build time via ldflags
var (
	Version     = "dev"
	GitHubOwner = "ajanitshimanga"    // Set via: -ldflags "-X 'OpenSmurfManager/internal/updater.GitHubOwner=username'"
	GitHubRepo  = "OpenSmurfManager" // Set via: -ldflags "-X 'OpenSmurfManager/internal/updater.GitHubRepo=repo'"
)

// Release represents a GitHub release
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
	HTMLURL     string    `json:"html_url"`
}

// Asset represents a release asset (downloadable file)
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseNotes   string `json:"releaseNotes"`
	DownloadURL    string `json:"downloadURL"`
	ReleaseURL     string `json:"releaseURL"`
	AssetSize      int64  `json:"assetSize"`
}

// Updater handles checking and applying updates
type Updater struct {
	owner   string
	repo    string
	current string
	client  *http.Client
}

// NewUpdater creates a new updater instance
func NewUpdater() *Updater {
	return &Updater{
		owner:   GitHubOwner,
		repo:    GitHubRepo,
		current: Version,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetCurrentVersion returns the current app version
func (u *Updater) GetCurrentVersion() string {
	return u.current
}

// CheckForUpdates checks GitHub for a newer release
func (u *Updater) CheckForUpdates() (*UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", u.owner, u.repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "OpenSmurfManager-Updater")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// No releases yet
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: u.current,
			LatestVersion:  u.current,
		}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(u.current, "v")

	// Find the appropriate asset for this platform
	assetName := u.getAssetName()
	var downloadURL string
	var assetSize int64

	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(assetName)) {
			downloadURL = asset.BrowserDownloadURL
			assetSize = asset.Size
			break
		}
	}

	info := &UpdateInfo{
		Available:      u.isNewerVersion(latestVersion, currentVersion),
		CurrentVersion: u.current,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		DownloadURL:    downloadURL,
		ReleaseURL:     release.HTMLURL,
		AssetSize:      assetSize,
	}

	return info, nil
}

// getAssetName returns the expected asset name for the current platform
func (u *Updater) getAssetName() string {
	switch runtime.GOOS {
	case "windows":
		return "Setup"
	case "darwin":
		return ".dmg"
	case "linux":
		return ".AppImage"
	default:
		return ""
	}
}

// isNewerVersion compares two semantic versions
func (u *Updater) isNewerVersion(latest, current string) bool {
	// Handle dev versions
	if current == "dev" || current == "" {
		return false // Don't prompt updates for dev builds
	}

	// Simple string comparison works for semantic versions like 1.0.0, 1.1.0, etc.
	// For more complex comparison, use a semver library
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		if latestParts[i] > currentParts[i] {
			return true
		} else if latestParts[i] < currentParts[i] {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}

// DownloadUpdate downloads the update to a temp file and returns the path
func (u *Updater) DownloadUpdate(downloadURL string, progressChan chan<- int) (string, error) {
	if downloadURL == "" {
		return "", fmt.Errorf("no download URL available")
	}

	resp, err := u.client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "OpenSmurfManager-Setup.exe")

	out, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	totalSize := resp.ContentLength
	downloaded := int64(0)
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return "", fmt.Errorf("failed to write: %w", writeErr)
			}
			downloaded += int64(n)

			if progressChan != nil && totalSize > 0 {
				progress := int(float64(downloaded) / float64(totalSize) * 100)
				select {
				case progressChan <- progress:
				default:
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("download error: %w", err)
		}
	}

	return tmpFile, nil
}

// ApplyUpdate runs the downloaded installer
func (u *Updater) ApplyUpdate(installerPath string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-update only supported on Windows")
	}

	// Run the installer with /SILENT flag for quiet install
	cmd := exec.Command(installerPath, "/SILENT", "/CLOSEAPPLICATIONS")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %w", err)
	}

	// Exit the current app to allow the installer to replace files
	os.Exit(0)
	return nil
}

// OpenReleasePage opens the release page in the default browser
func (u *Updater) OpenReleasePage(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}
