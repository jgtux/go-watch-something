package subtitles

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

// Subliminal fetches subtitles via the external `subliminal` CLI
// (https://github.com/Diaoul/subliminal). Requires it to be installed
// separately -- see README.
type Subliminal struct{}

func (Subliminal) Name() string { return "subliminal" }

func (Subliminal) Fetch(videoDir string, langs []string) error {
	if _, err := exec.LookPath("subliminal"); err != nil {
		return fmt.Errorf("%w: subliminal binary not found on PATH", ErrNotConfigured)
	}

	fmt.Println("Fetching subtitles via subliminal...")

	absPath, err := filepath.Abs(videoDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	args := []string{"download"}
	for _, lang := range langs {
		args = append(args, "-l", lang)
	}
	args = append(args, absPath)

	cmd := exec.Command("subliminal", args...)
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("subliminal download failed: %w\nstderr: %s", err, stderr.String())
	}

	fmt.Println("Subtitles downloaded successfully via subliminal.")
	return nil
}
