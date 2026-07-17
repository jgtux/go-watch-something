// Package player launches a video player pointed at the local stream URL,
// for -autoplay. xdg-open works on both Linux and BSD (not Linux-only),
// so it's tried first; mpv/vlc are the fallback for systems without it.
package player

import (
	"errors"
	"os/exec"
)

// ErrNoPlayerFound is returned when override is empty and none of the
// candidate players are on PATH. Callers should fall back to printing the
// stream URL for the user to open manually.
var ErrNoPlayerFound = errors.New("player: no video player found (tried xdg-open, mpv, vlc); pass -player to specify one")

var candidates = []string{"xdg-open", "mpv", "vlc"}

// Launch opens url in a video player without blocking on it exiting. If
// override is non-empty it's used directly (and must resolve via
// exec.LookPath). Otherwise the first of xdg-open/mpv/vlc found on PATH is
// used.
func Launch(url, override string) error {
	bin := override
	if bin == "" {
		bin = find()
		if bin == "" {
			return ErrNoPlayerFound
		}
	}
	if _, err := exec.LookPath(bin); err != nil {
		return err
	}
	return exec.Command(bin, url).Start()
}

func find() string {
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}
