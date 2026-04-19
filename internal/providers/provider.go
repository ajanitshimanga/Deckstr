// Package providers defines the interface for game platform integrations.
//
// A Provider encapsulates all platform-specific logic for detecting signed-in
// accounts, matching them to stored accounts, and updating stored data. New
// platforms (Epic, Steam, Battle.net, etc.) are added by implementing this
// interface in a sub-package and registering with the Registry.
package providers

import (
	"context"
	"time"

	"OpenSmurfManager/internal/models"
)

// DetectedAccount represents a currently signed-in account detected from
// any supported platform. Providers populate the universal fields for all
// platforms. Platform-specific fields (e.g., RiotID) are only populated by
// the owning provider.
type DetectedAccount struct {
	// Universal fields - all providers must populate these
	NetworkID   string    // Platform identifier ("riot", "epic", etc.)
	DisplayName string    // Human-readable label (e.g., "turkish aimer#doner")
	UniqueID    string    // Platform's unique account identifier
	DetectedAt  time.Time // When detection occurred

	// Optional generic game data
	Ranks        []models.CachedRank
	TopMasteries []models.ChampionMastery

	// Riot-specific fields (populated only when NetworkID == "riot")
	RiotID        string
	PUUID         string
	GameName      string
	TagLine       string
	SummonerLevel int
}

// DetectionError indicates a detection attempt failed. Providers should return
// this type (via errors.As) so callers can distinguish "client not running"
// (expected) from actual errors.
type DetectionError struct {
	Code    string // Stable error code (e.g., "client_offline")
	Message string // Human-readable message
	Retry   bool   // Whether the caller should retry later
}

func (e *DetectionError) Error() string {
	return e.Message
}

// Standard error codes that all providers should use where applicable.
const (
	ErrCodeClientOffline = "client_offline"  // No client process running
	ErrCodeNotSignedIn   = "not_signed_in"   // Client running but no active session
	ErrCodeFetchFailed   = "fetch_failed"    // Failed to retrieve account details
	ErrCodeUnsupportedOS = "unsupported_os"  // Platform not supported on this OS
)

// Provider is the interface all game platform integrations must implement.
//
// Implementations should be stateless where possible - each method may be
// called concurrently. Expensive resources (HTTP clients, etc.) can be held
// on the implementing struct but must be safe for concurrent use.
type Provider interface {
	// NetworkID returns the stable identifier for this platform.
	// Must match the NetworkID used on Account and GameNetwork.
	// Example: "riot", "epic", "steam"
	NetworkID() string

	// DisplayName returns a human-readable platform name for UI.
	// Example: "Riot Games", "Epic Games"
	DisplayName() string

	// IsClientRunning returns true if a detectable client for this platform
	// is currently running. Should be cheap (process check, not API call).
	IsClientRunning(ctx context.Context) bool

	// Detect attempts to identify the currently signed-in account.
	//
	// Returns (nil, *DetectionError) when no client is running or no session
	// exists - these are expected states, not errors. Callers should check
	// for DetectionError via errors.As before treating as a failure.
	//
	// Returns a populated DetectedAccount on success. The NetworkID field
	// must be set to match this provider's NetworkID().
	Detect(ctx context.Context) (*DetectedAccount, error)

	// MatchAccount finds the stored account corresponding to a detected
	// session. Returns nil if no match is found.
	//
	// Matching strategy is platform-specific (PUUID for Riot, email for
	// Epic, etc.) - providers should use the most reliable identifier
	// available and fall back to weaker matches as needed.
	MatchAccount(accounts []models.Account, detected *DetectedAccount) *models.Account

	// UpdateAccount applies detected session data to a stored account.
	// This is where ranks, masteries, and cached identifiers get persisted.
	//
	// The account pointer is mutated in place. The caller is responsible
	// for persisting the updated account via the storage layer.
	UpdateAccount(account *models.Account, detected *DetectedAccount)
}
