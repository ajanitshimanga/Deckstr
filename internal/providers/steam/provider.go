// Package steam implements the providers.Provider interface for the Steam
// storefront. It currently covers Rocket League but will extend to other
// Steam titles as they're added to models.DefaultGameNetworks.
//
// Steam exposes persistent user data under a local directory (loginusers.vdf,
// config.vdf), but parsing those is deferred: this first cut reports
// launcher presence only and returns a structured DetectionError from
// Detect so the registry can move on to the next provider.
package steam

import (
	"context"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/process"
	"OpenSmurfManager/internal/providers"
)

// NetworkID is the stable identifier for the Steam provider.
const NetworkID = "steam"

// Provider implements providers.Provider for Steam titles.
type Provider struct{}

// New returns a new Steam provider instance.
func New() *Provider {
	return &Provider{}
}

// NetworkID returns "steam".
func (p *Provider) NetworkID() string {
	return NetworkID
}

// DisplayName returns "Steam".
func (p *Provider) DisplayName() string {
	return "Steam"
}

// IsClientRunning checks for the Steam client process. A running game (e.g.
// RocketLeague.exe) is tracked separately via models.Game.GameProcesses.
func (p *Provider) IsClientRunning(_ context.Context) bool {
	return process.AnyRunning(collectClientProcesses())
}

// Detect cannot yet resolve the signed-in Steam account without parsing
// loginusers.vdf or talking to a Steam Web API key, so it returns a stable
// DetectionError. The registry treats that as "client seen but session
// unknown" and moves on.
func (p *Provider) Detect(ctx context.Context) (*providers.DetectedAccount, error) {
	if !p.IsClientRunning(ctx) {
		return nil, &providers.DetectionError{
			Code:    providers.ErrCodeClientOffline,
			Message: "Steam is not running",
			Retry:   true,
		}
	}
	return nil, &providers.DetectionError{
		Code:    providers.ErrCodeNotSignedIn,
		Message: "Steam account auto-detection is not yet supported",
		Retry:   false,
	}
}

// MatchAccount returns nil because we have no reliable Steam-side identifier
// to match against yet. Users still get value from manually tagging accounts
// with NetworkID="steam" — the rest of the app treats those as first-class.
func (p *Provider) MatchAccount(_ []models.Account, _ *providers.DetectedAccount) *models.Account {
	return nil
}

// UpdateAccount is a noop — with no Detect payload we have nothing to merge.
func (p *Provider) UpdateAccount(_ *models.Account, _ *providers.DetectedAccount) {}

// collectClientProcesses gathers all Steam launcher process names for the
// current platform from the default game network definitions.
func collectClientProcesses() []string {
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
