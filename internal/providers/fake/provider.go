// Package fake provides a configurable in-memory Provider implementation
// for use in service integration tests. Test authors construct a Provider
// with the desired detection state and inject it into the registry.
//
// This is the canonical fake for E2E tests - prefer it over per-test
// duplicates so that contract changes propagate to all callers.
package fake

import (
	"context"
	"sync/atomic"

	"OpenSmurfManager/internal/models"
	"OpenSmurfManager/internal/providers"
)

// Provider is a configurable fake. Set fields on the struct to control
// behavior; counters and call records are read-safe via atomics/mutex.
type Provider struct {
	ID            string
	Name          string
	ClientRunning bool
	Detected      *providers.DetectedAccount
	DetectErr     error

	// MatchFunc, if set, overrides the default match. The default matches
	// purely on Account.NetworkID == provider.ID && Account.ID == detected.UniqueID,
	// using only fields from the generic Provider contract - no platform-specific
	// identifiers. Set this to simulate platform-specific matching strategies
	// (e.g. PUUID for Riot, email for Epic).
	MatchFunc func(accounts []models.Account, detected *providers.DetectedAccount) *models.Account

	// UpdateFunc, if set, runs in addition to the default field copy.
	// Use this to simulate side effects like rank merging.
	UpdateFunc func(account *models.Account, detected *providers.DetectedAccount)

	detectCalls   int64
	matchCalls    int64
	updateCalls   int64
	runningChecks int64
}

// New returns a fake provider with the given NetworkID and DisplayName.
// Other fields default to zero (not running, no detected account).
func New(id, name string) *Provider {
	return &Provider{ID: id, Name: name}
}

// NetworkID returns the configured ID.
func (f *Provider) NetworkID() string { return f.ID }

// DisplayName returns the configured display name.
func (f *Provider) DisplayName() string { return f.Name }

// IsClientRunning returns the configured ClientRunning flag and increments
// the call counter.
func (f *Provider) IsClientRunning(_ context.Context) bool {
	atomic.AddInt64(&f.runningChecks, 1)
	return f.ClientRunning
}

// Detect returns the configured Detected/DetectErr pair.
func (f *Provider) Detect(_ context.Context) (*providers.DetectedAccount, error) {
	atomic.AddInt64(&f.detectCalls, 1)
	if f.DetectErr != nil {
		return nil, f.DetectErr
	}
	return f.Detected, nil
}

// MatchAccount runs MatchFunc if set, otherwise matches purely on the
// generic contract fields (Account.NetworkID == f.ID && Account.ID ==
// detected.UniqueID). It deliberately ignores platform-specific fields
// like PUUID or RiotID - tests that need richer matching should provide
// MatchFunc.
func (f *Provider) MatchAccount(accounts []models.Account, detected *providers.DetectedAccount) *models.Account {
	atomic.AddInt64(&f.matchCalls, 1)
	if detected == nil {
		return nil
	}
	if f.MatchFunc != nil {
		return f.MatchFunc(accounts, detected)
	}
	for i := range accounts {
		if accounts[i].NetworkID != f.ID {
			continue
		}
		if detected.UniqueID != "" && accounts[i].ID == detected.UniqueID {
			return &accounts[i]
		}
	}
	return nil
}

// UpdateAccount records the call and applies a generic field copy from
// detected to account (CachedRanks, TopMasteries). Platform-specific
// fields are not touched by the default - use UpdateFunc to simulate
// e.g. PUUID caching.
func (f *Provider) UpdateAccount(account *models.Account, detected *providers.DetectedAccount) {
	atomic.AddInt64(&f.updateCalls, 1)
	if account == nil || detected == nil {
		return
	}

	if len(detected.Ranks) > 0 {
		account.CachedRanks = detected.Ranks
	}
	if len(detected.TopMasteries) > 0 {
		account.TopMasteries = detected.TopMasteries
	}

	if f.UpdateFunc != nil {
		f.UpdateFunc(account, detected)
	}
}

// Stats returns counter snapshots for assertion in tests.
func (f *Provider) Stats() Stats {
	return Stats{
		DetectCalls:   atomic.LoadInt64(&f.detectCalls),
		MatchCalls:    atomic.LoadInt64(&f.matchCalls),
		UpdateCalls:   atomic.LoadInt64(&f.updateCalls),
		RunningChecks: atomic.LoadInt64(&f.runningChecks),
	}
}

// Stats is a snapshot of fake provider call counters.
type Stats struct {
	DetectCalls   int64
	MatchCalls    int64
	UpdateCalls   int64
	RunningChecks int64
}
