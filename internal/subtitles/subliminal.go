package subtitles

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

func Fetch(file string, langs []string) error {
	fmt.Println("Fetching subtitles...")

	absPath, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("Failed to get absolute path: %v", err)
	}

	args := []string{"download"}
	for _, lang := range langs {
		args = append(args, "-l", lang)
	}
	// Pass the absolute path as a single argument
	args = append(args, absPath)

	cmd := exec.Command("subliminal", args...)
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Subtitles download failed: %v\nStderr: %s", err, stderr.String())
	}

	fmt.Println("Subtitles downloaded successfully.")
	return nil
}
