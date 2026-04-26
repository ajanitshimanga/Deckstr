package telemetry

import (
	"github.com/posthog/posthog-go"
)

// posthogAPIKey is the PostHog project key. Empty by default so the source
// repo never carries a key; populated at link time via:
//   go build -ldflags "-X OpenSmurfManager/internal/telemetry.posthogAPIKey=phc_xxx"
// When empty, no events are shipped (local file logging still works).
var posthogAPIKey string

// posthogEndpoint is the ingest host. Override per region (EU users would
// set "https://eu.i.posthog.com") via the same -X ldflag mechanism.
var posthogEndpoint = "https://us.i.posthog.com"

// posthogSkipEvents are event names that fire on a polling cadence and would
// dominate the event budget. Local logging still records them; only the
// network shipper drops them. Adjust by setting Options.PostHogSkipEvents.
var defaultPosthogSkipEvents = map[string]bool{
	"account.detect": true,
}

// posthogShipper forwards records to PostHog. Nil-safe: a zero-value
// pointer ignores all calls, so callers don't need to nil-check.
type posthogShipper struct {
	client     posthog.Client
	distinctID string
	skip       map[string]bool
}

// newPosthogShipper returns nil if no API key is configured, signalling
// "no shipping" without requiring callers to branch.
func newPosthogShipper(apiKey, endpoint, distinctID string, skip map[string]bool) *posthogShipper {
	if apiKey == "" || distinctID == "" {
		return nil
	}
	client, err := posthog.NewWithConfig(apiKey, posthog.Config{
		Endpoint: endpoint,
	})
	if err != nil {
		return nil
	}
	return &posthogShipper{
		client:     client,
		distinctID: distinctID,
		skip:       skip,
	}
}

// capture enqueues one event. The PostHog SDK is async + batched, so this
// returns quickly and never blocks the caller. Errors are swallowed.
func (s *posthogShipper) capture(event string, properties map[string]interface{}) {
	if s == nil || s.client == nil {
		return
	}
	if s.skip[event] {
		return
	}
	_ = s.client.Enqueue(posthog.Capture{
		DistinctId: s.distinctID,
		Event:      event,
		Properties: properties,
	})
}

func (s *posthogShipper) close() {
	if s == nil || s.client == nil {
		return
	}
	_ = s.client.Close()
}
