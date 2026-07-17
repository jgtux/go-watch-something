package subtitles

import (
	"os"
	"testing"
)

func TestSubliminal_NotConfiguredWhenBinaryMissing(t *testing.T) {
	original := os.Getenv("PATH")
	os.Setenv("PATH", t.TempDir()) // empty dir -- subliminal definitely not here
	defer os.Setenv("PATH", original)

	err := Subliminal{}.Fetch(t.TempDir(), []string{"en"})
	if err == nil {
		t.Fatal("Fetch with no subliminal on PATH = nil, want error")
	}
	if !isNotConfigured(err) {
		t.Errorf("Fetch with no subliminal on PATH = %v, want it to wrap ErrNotConfigured", err)
	}
}
