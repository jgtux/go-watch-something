package subtitles

import (
	"errors"
	"testing"
)

type fakeProvider struct {
	name string
	err  error
}

func (f fakeProvider) Name() string                 { return f.name }
func (f fakeProvider) Fetch(string, []string) error { return f.err }

func TestFetchWithFallback_FirstSuccessWins(t *testing.T) {
	calledSecond := false
	providers := []Provider{
		fakeProvider{name: "first", err: nil},
		fakeProviderFunc{name: "second", fn: func() error { calledSecond = true; return nil }},
	}

	if err := FetchWithFallback(providers, "/tmp/whatever", []string{"en"}); err != nil {
		t.Fatalf("FetchWithFallback: %v", err)
	}
	if calledSecond {
		t.Errorf("second provider was called even though the first succeeded")
	}
}

func TestFetchWithFallback_FallsThroughOnNotConfigured(t *testing.T) {
	providers := []Provider{
		fakeProvider{name: "unconfigured", err: ErrNotConfigured},
		fakeProvider{name: "works", err: nil},
	}

	if err := FetchWithFallback(providers, "/tmp/whatever", []string{"en"}); err != nil {
		t.Fatalf("FetchWithFallback: %v", err)
	}
}

func TestFetchWithFallback_AllFail(t *testing.T) {
	providers := []Provider{
		fakeProvider{name: "a", err: errors.New("boom a")},
		fakeProvider{name: "b", err: errors.New("boom b")},
	}

	err := FetchWithFallback(providers, "/tmp/whatever", []string{"en"})
	if err == nil {
		t.Fatal("FetchWithFallback = nil, want error when every provider fails")
	}
}

func TestFetchWithFallback_NoProviders(t *testing.T) {
	err := FetchWithFallback(nil, "/tmp/whatever", []string{"en"})
	if err == nil {
		t.Fatal("FetchWithFallback with no providers = nil, want error")
	}
}

// fakeProviderFunc lets a test observe whether Fetch was actually called.
type fakeProviderFunc struct {
	name string
	fn   func() error
}

func (f fakeProviderFunc) Name() string                 { return f.name }
func (f fakeProviderFunc) Fetch(string, []string) error { return f.fn() }
