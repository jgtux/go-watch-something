// Package subtitles fetches subtitle files for a downloaded video,
// trying multiple providers in order so one provider's absence or failure
// doesn't mean giving up on subtitles entirely.
package subtitles

import (
	"fmt"
	"strings"
)

// Provider fetches subtitle files for the video(s) in videoDir, in the
// given languages, writing them next to the video. ErrNotConfigured
// signals the provider is unavailable in this environment (missing
// binary, missing API key, ...) rather than a real failure -- callers
// move on to the next provider without logging it as an error.
type Provider interface {
	Name() string
	Fetch(videoDir string, langs []string) error
}

// ErrNotConfigured is returned by a Provider whose prerequisites (a
// binary on PATH, an API key, ...) aren't met.
var ErrNotConfigured = fmt.Errorf("subtitles: provider not configured")

// FetchWithFallback tries each provider in order, returning nil on the
// first success. If every provider fails or is unconfigured, it returns
// an error summarizing what was tried.
func FetchWithFallback(providers []Provider, videoDir string, langs []string) error {
	var failures []string
	for _, p := range providers {
		err := p.Fetch(videoDir, langs)
		if err == nil {
			return nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", p.Name(), err))
	}
	if len(providers) == 0 {
		return fmt.Errorf("subtitles: no providers configured")
	}
	return fmt.Errorf("subtitles: all providers failed:\n%s", strings.Join(failures, "\n"))
}
