// Package epic implements the providers.Provider interface for the Epic
// Games storefront. It currently covers Rocket League but will extend to
// other Epic-distributed titles as they're added to models.DefaultGameNetworks.
//
// Unlike the Riot provider, Epic does not expose a local LCU-style API we
// can query for the signed-in account, so Detect reports a stable
// "not signed in" DetectionError even when the launcher is running. The
// launcher presence is still surfaced via IsClientRunning so the polling
// loop can keep the UI in the "client active" state while the user decides
// who is playing.
package epic

import (
	"context"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/process"
	"OpenSmurfManager/internal/providers"
)

// NetworkID is the stable identifier for the Epic Games provider.
const NetworkID = "epic"

// Provider implements providers.Provider for Epic Games titles.
type Provider struct{}

// New returns a new Epic provider instance.
func New() *Provider {
	return &Provider{}
}

// NetworkID returns "epic".
func (p *Provider) NetworkID() string {
	return NetworkID
}

// DisplayName returns "Epic Games".
func (p *Provider) DisplayName() string {
	return "Epic Games"
}

// IsClientRunning checks for the Epic Games launcher process. Running titles
// like Rocket League are reported via models.Game.GameProcesses and handled
// separately by app.IsInGame.
func (p *Provider) IsClientRunning(_ context.Context) bool {
	return process.AnyRunning(collectClientProcesses())
}

// Detect currently cannot identify the signed-in Epic account without OAuth
// or filesystem scraping, so it always returns a structured DetectionError.
// Callers treat this as "platform seen but session unknown" and fall through
// to the next provider in the registry.
func (p *Provider) Detect(ctx context.Context) (*providers.DetectedAccount, error) {
	if !p.IsClientRunning(ctx) {
		return nil, &providers.DetectionError{
			Code:    providers.ErrCodeClientOffline,
			Message: "Epic Games Launcher is not running",
			Retry:   true,
		}
	}
	return nil, &providers.DetectionError{
		Code:    providers.ErrCodeNotSignedIn,
		Message: "Epic Games account auto-detection is not yet supported",
		Retry:   false,
	}
}

// MatchAccount returns nil because we have no reliable Epic-side identifier
// to match against yet. Users still get value from the provider by manually
// tagging accounts with NetworkID="epic" — the rest of the app treats those
// as first-class entries.
func (p *Provider) MatchAccount(_ []models.Account, _ *providers.DetectedAccount) *models.Account {
	return nil
}

// UpdateAccount is a noop — with no Detect payload we have nothing to merge.
func (p *Provider) UpdateAccount(_ *models.Account, _ *providers.DetectedAccount) {}

// collectClientProcesses gathers all Epic launcher process names for the
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
