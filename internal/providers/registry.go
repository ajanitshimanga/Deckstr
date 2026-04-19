package providers

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"OpenSmurfManager/internal/models"
)

// Registry holds all registered platform providers, keyed by NetworkID.
//
// The registry is the single integration point for app.go - detection,
// matching, and updates are dispatched through it. Adding a new platform
// requires only registering a new Provider implementation.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry. Providers are keyed by their
// NetworkID(). Registering a duplicate NetworkID returns an error.
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return errors.New("cannot register nil provider")
	}

	id := p.NetworkID()
	if id == "" {
		return errors.New("provider NetworkID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[id]; exists {
		return fmt.Errorf("provider %q already registered", id)
	}

	r.providers[id] = p
	return nil
}

// MustRegister is like Register but panics on error. Intended for use during
// initialization where a registration failure is a programmer error.
func (r *Registry) MustRegister(p Provider) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

// Get returns the provider for the given NetworkID, or nil if not registered.
func (r *Registry) Get(networkID string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[networkID]
}

// All returns a snapshot of all registered providers. Iteration order is
// not guaranteed to be stable between calls.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}

// DetectAny tries each registered provider in turn and returns the first
// successful detection. Providers that return a DetectionError (client
// offline, not signed in) are skipped - only unexpected errors are returned.
//
// Returns (nil, nil) if no provider has an active session but none errored.
// Returns (nil, err) if at least one provider hit an unexpected failure.
func (r *Registry) DetectAny(ctx context.Context) (*DetectedAccount, error) {
	r.mu.RLock()
	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	r.mu.RUnlock()

	var firstRealError error
	for _, p := range providers {
		detected, err := p.Detect(ctx)
		if err == nil && detected != nil {
			return detected, nil
		}

		// Skip expected "not running / not signed in" states
		var detErr *DetectionError
		if errors.As(err, &detErr) {
			continue
		}

		// Record first unexpected error, but keep trying other providers
		if err != nil && firstRealError == nil {
			firstRealError = err
		}
	}

	return nil, firstRealError
}

// MatchAccount dispatches matching to the provider identified by the detected
// account's NetworkID. Returns nil if the provider is unknown or no match.
func (r *Registry) MatchAccount(accounts []models.Account, detected *DetectedAccount) *models.Account {
	if detected == nil {
		return nil
	}

	p := r.Get(detected.NetworkID)
	if p == nil {
		return nil
	}

	return p.MatchAccount(accounts, detected)
}

// UpdateAccount dispatches account updates to the owning provider. No-op if
// the provider is unknown.
func (r *Registry) UpdateAccount(account *models.Account, detected *DetectedAccount) {
	if detected == nil || account == nil {
		return
	}

	p := r.Get(detected.NetworkID)
	if p == nil {
		return
	}

	p.UpdateAccount(account, detected)
}

// IsAnyClientRunning returns true if any registered provider reports a
// running client. Useful for polling decisions.
func (r *Registry) IsAnyClientRunning(ctx context.Context) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		if p.IsClientRunning(ctx) {
			return true
		}
	}
	return false
}
