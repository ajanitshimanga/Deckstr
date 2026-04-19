package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"OpenSmurfManager/internal/models"
)

// fakeProvider is a configurable provider for testing.
type fakeProvider struct {
	id           string
	name         string
	running      bool
	detected     *DetectedAccount
	detectErr    error
	matched      *models.Account
	updatesCount int
}

func (f *fakeProvider) NetworkID() string                    { return f.id }
func (f *fakeProvider) DisplayName() string                  { return f.name }
func (f *fakeProvider) IsClientRunning(_ context.Context) bool { return f.running }
func (f *fakeProvider) Detect(_ context.Context) (*DetectedAccount, error) {
	return f.detected, f.detectErr
}
func (f *fakeProvider) MatchAccount(_ []models.Account, _ *DetectedAccount) *models.Account {
	return f.matched
}
func (f *fakeProvider) UpdateAccount(_ *models.Account, _ *DetectedAccount) {
	f.updatesCount++
}

func TestRegisterRejectsNil(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Fatal("expected error registering nil provider")
	}
}

func TestRegisterRejectsEmptyNetworkID(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&fakeProvider{id: ""}); err == nil {
		t.Fatal("expected error registering provider with empty NetworkID")
	}
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&fakeProvider{id: "riot"}); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := r.Register(&fakeProvider{id: "riot"}); err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestGetReturnsNilForUnknown(t *testing.T) {
	r := NewRegistry()
	if got := r.Get("nope"); got != nil {
		t.Fatalf("expected nil for unknown provider, got %#v", got)
	}
}

func TestDetectAnyReturnsFirstSuccess(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&fakeProvider{
		id: "offline",
		detectErr: &DetectionError{
			Code:    ErrCodeClientOffline,
			Message: "not running",
		},
	})
	r.MustRegister(&fakeProvider{
		id: "online",
		detected: &DetectedAccount{
			NetworkID:   "online",
			DisplayName: "found me",
			DetectedAt:  time.Now(),
		},
	})

	got, err := r.DetectAny(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.DisplayName != "found me" {
		t.Fatalf("expected to detect online provider, got %#v", got)
	}
}

func TestDetectAnyReturnsNilWhenAllOffline(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&fakeProvider{
		id: "a",
		detectErr: &DetectionError{Code: ErrCodeClientOffline, Message: "offline"},
	})
	r.MustRegister(&fakeProvider{
		id: "b",
		detectErr: &DetectionError{Code: ErrCodeNotSignedIn, Message: "no session"},
	})

	got, err := r.DetectAny(context.Background())
	if err != nil {
		t.Fatalf("expected no error when all providers cleanly offline, got: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil detection, got %#v", got)
	}
}

func TestDetectAnySurfacesUnexpectedError(t *testing.T) {
	r := NewRegistry()
	boom := errors.New("network on fire")
	r.MustRegister(&fakeProvider{id: "a", detectErr: boom})

	_, err := r.DetectAny(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected real error to surface, got %v", err)
	}
}

func TestMatchAccountDispatchesByNetworkID(t *testing.T) {
	r := NewRegistry()
	expected := &models.Account{ID: "acc-1"}
	r.MustRegister(&fakeProvider{id: "riot", matched: expected})
	r.MustRegister(&fakeProvider{id: "epic", matched: nil})

	got := r.MatchAccount(nil, &DetectedAccount{NetworkID: "riot"})
	if got != expected {
		t.Fatalf("expected dispatch to riot provider, got %#v", got)
	}

	got = r.MatchAccount(nil, &DetectedAccount{NetworkID: "epic"})
	if got != nil {
		t.Fatalf("expected nil match from epic provider, got %#v", got)
	}

	got = r.MatchAccount(nil, &DetectedAccount{NetworkID: "unknown"})
	if got != nil {
		t.Fatalf("expected nil match for unknown provider, got %#v", got)
	}
}

func TestUpdateAccountDispatchesByNetworkID(t *testing.T) {
	r := NewRegistry()
	riotProv := &fakeProvider{id: "riot"}
	epicProv := &fakeProvider{id: "epic"}
	r.MustRegister(riotProv)
	r.MustRegister(epicProv)

	r.UpdateAccount(&models.Account{}, &DetectedAccount{NetworkID: "epic"})
	if epicProv.updatesCount != 1 {
		t.Fatalf("expected epic provider to receive update, got count=%d", epicProv.updatesCount)
	}
	if riotProv.updatesCount != 0 {
		t.Fatalf("expected riot provider to be untouched, got count=%d", riotProv.updatesCount)
	}
}

func TestIsAnyClientRunning(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&fakeProvider{id: "a", running: false})
	if r.IsAnyClientRunning(context.Background()) {
		t.Fatal("expected no clients running")
	}

	r.MustRegister(&fakeProvider{id: "b", running: true})
	if !r.IsAnyClientRunning(context.Background()) {
		t.Fatal("expected at least one client running")
	}
}
