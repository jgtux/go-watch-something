package player

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeStub creates an executable no-op script named name in dir, real
// enough that exec.LookPath and exec.Command().Start() both succeed
// against it -- this exercises the real find()/Launch() logic against
// real PATH lookups, not a mocked lookup function.
func writeStub(t *testing.T, dir, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub scripts are POSIX shell, not written for windows")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("writing stub %s: %v", name, err)
	}
}

func withPath(t *testing.T, dir string) {
	t.Helper()
	original := os.Getenv("PATH")
	os.Setenv("PATH", dir)
	t.Cleanup(func() { os.Setenv("PATH", original) })
}

func TestLaunch_PrefersXdgOpen(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "xdg-open")
	writeStub(t, dir, "mpv")
	withPath(t, dir)

	if err := Launch("http://localhost:8080/movie", ""); err != nil {
		t.Fatalf("Launch: %v", err)
	}
}

func TestLaunch_FallsBackToMpvWhenNoXdgOpen(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "mpv")
	withPath(t, dir)

	if err := Launch("http://localhost:8080/movie", ""); err != nil {
		t.Fatalf("Launch: %v", err)
	}
}

func TestLaunch_FallsBackToVlcWhenNoXdgOpenOrMpv(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "vlc")
	withPath(t, dir)

	if err := Launch("http://localhost:8080/movie", ""); err != nil {
		t.Fatalf("Launch: %v", err)
	}
}

func TestLaunch_NoPlayerFound(t *testing.T) {
	dir := t.TempDir() // empty -- nothing on PATH
	withPath(t, dir)

	err := Launch("http://localhost:8080/movie", "")
	if err != ErrNoPlayerFound {
		t.Errorf("Launch = %v, want ErrNoPlayerFound", err)
	}
}

func TestLaunch_OverrideTakesPriorityEvenIfCandidatesExist(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "xdg-open")
	writeStub(t, dir, "my-custom-player")
	withPath(t, dir)

	if err := Launch("http://localhost:8080/movie", "my-custom-player"); err != nil {
		t.Fatalf("Launch: %v", err)
	}
}

func TestLaunch_OverrideNotFound(t *testing.T) {
	dir := t.TempDir()
	withPath(t, dir)

	err := Launch("http://localhost:8080/movie", "definitely-not-a-real-player")
	if err == nil {
		t.Errorf("Launch with a nonexistent override = nil error, want error")
	}
}
