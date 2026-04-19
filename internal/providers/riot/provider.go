// Package riot implements the providers.Provider interface for Riot Games.
//
// This package is a thin adapter over internal/riotclient - all LCU protocol
// and Riot API logic remains in that package. This adapter translates between
// the generic providers.DetectedAccount and Riot's native types.
package riot

import (
	"context"
	"errors"
	"strings"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/process"
	"OpenSmurfManager/internal/providers"
	"OpenSmurfManager/internal/riotclient"
)

// NetworkID is the stable identifier for the Riot Games provider.
const NetworkID = "riot"

// Provider implements providers.Provider for Riot Games platforms
// (League of Legends, Teamfight Tactics, Valorant).
type Provider struct{}

// New returns a new Riot provider instance.
func New() *Provider {
	return &Provider{}
}

// NetworkID returns "riot".
func (p *Provider) NetworkID() string {
	return NetworkID
}

// DisplayName returns "Riot Games".
func (p *Provider) DisplayName() string {
	return "Riot Games"
}

// IsClientRunning checks for any Riot client process. This is a cheap
// process check and does not attempt to connect to the LCU API.
func (p *Provider) IsClientRunning(_ context.Context) bool {
	clientProcesses := collectRiotClientProcesses()
	return process.AnyRunning(clientProcesses)
}

// Detect connects to the LCU and fetches the signed-in account details,
// including ranks and top masteries.
func (p *Provider) Detect(_ context.Context) (*providers.DetectedAccount, error) {
	detected, err := riotclient.DetectAndFetchRanks()
	if err != nil {
		return nil, translateError(err)
	}

	if detected == nil {
		return nil, &providers.DetectionError{
			Code:    providers.ErrCodeNotSignedIn,
			Message: "No signed-in Riot account detected",
			Retry:   true,
		}
	}

	return fromRiotDetected(detected), nil
}

// MatchAccount tries to locate the stored account by PUUID (most reliable),
// falling back to case-insensitive RiotID comparison.
func (p *Provider) MatchAccount(accounts []models.Account, detected *providers.DetectedAccount) *models.Account {
	if detected == nil {
		return nil
	}

	if detected.PUUID != "" {
		if matched := riotclient.MatchAccountByPUUID(accounts, detected.PUUID); matched != nil {
			return matched
		}
	}

	if detected.RiotID != "" {
		return riotclient.MatchAccountByRiotID(accounts, detected.RiotID)
	}

	return nil
}

// UpdateAccount applies detected ranks and identifiers to the stored account.
// Delegates to riotclient.UpdateAccountRanks which handles the merge logic.
func (p *Provider) UpdateAccount(account *models.Account, detected *providers.DetectedAccount) {
	if account == nil || detected == nil {
		return
	}

	native := toRiotDetected(detected)
	riotclient.UpdateAccountRanks(account, native)
}

// fromRiotDetected converts the native riotclient.DetectedAccount into the
// generic providers.DetectedAccount, preserving Riot-specific fields.
func fromRiotDetected(src *riotclient.DetectedAccount) *providers.DetectedAccount {
	return &providers.DetectedAccount{
		NetworkID:     NetworkID,
		DisplayName:   src.RiotID,
		UniqueID:      src.PUUID,
		DetectedAt:    src.DetectedAt,
		Ranks:         src.Ranks,
		TopMasteries:  src.TopMasteries,
		RiotID:        src.RiotID,
		PUUID:         src.PUUID,
		GameName:      src.GameName,
		TagLine:       src.TagLine,
		SummonerLevel: src.SummonerLevel,
	}
}

// toRiotDetected converts a generic DetectedAccount back to the native type
// for use with riotclient helpers.
func toRiotDetected(src *providers.DetectedAccount) *riotclient.DetectedAccount {
	return &riotclient.DetectedAccount{
		RiotID:        src.RiotID,
		GameName:      src.GameName,
		TagLine:       src.TagLine,
		PUUID:         src.PUUID,
		SummonerLevel: src.SummonerLevel,
		Ranks:         src.Ranks,
		TopMasteries:  src.TopMasteries,
		DetectedAt:    src.DetectedAt,
	}
}

// translateError converts riotclient.DetectionError into the generic
// providers.DetectionError so callers can treat all provider errors uniformly.
func translateError(err error) error {
	var riotErr *riotclient.DetectionError
	if !errors.As(err, &riotErr) {
		return err
	}

	return &providers.DetectionError{
		Code:    mapErrorCode(riotErr.Code),
		Message: riotErr.Message,
		Retry:   riotErr.Retry,
	}
}

// mapErrorCode translates riotclient-specific codes to standard provider codes.
func mapErrorCode(riotCode string) string {
	switch riotCode {
	case "client_offline", "lockfile_not_found":
		return providers.ErrCodeClientOffline
	case "summoner_fetch_failed":
		return providers.ErrCodeFetchFailed
	default:
		return strings.ToLower(riotCode)
	}
}

// collectRiotClientProcesses gathers all Riot client process names for the
// current platform from the default game network definitions.
func collectRiotClientProcesses() []string {
	var names []string
	for _, network := range models.DefaultGameNetworks() {
		if network.ID != NetworkID {
			continue
		}
		for _, game := range network.Games {
			names = append(names, game.ClientProcess.ForCurrentPlatform()...)
		}
	}
	return names
}
