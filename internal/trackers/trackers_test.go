package trackers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testMagnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"

func TestAddTrackers_FromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("udp://tracker.one:80\n# a comment\nudp://tracker.two:80\n\n"))
	}))
	defer srv.Close()

	got, err := AddTrackers(testMagnet, srv.URL)
	if err != nil {
		t.Fatalf("AddTrackers: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("result is not a valid URL: %v", err)
	}
	trs := u.Query()["tr"]
	if len(trs) != 2 {
		t.Fatalf("got %d trackers, want 2: %v", len(trs), trs)
	}
	if trs[0] != "udp://tracker.one:80" || trs[1] != "udp://tracker.two:80" {
		t.Errorf("trackers = %v, want [udp://tracker.one:80 udp://tracker.two:80]", trs)
	}
}

func TestAddTrackers_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trackers.txt")
	if err := os.WriteFile(path, []byte("udp://from-file:80\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := AddTrackers(testMagnet, path)
	if err != nil {
		t.Fatalf("AddTrackers: %v", err)
	}

	u, _ := url.Parse(got)
	trs := u.Query()["tr"]
	if len(trs) != 1 || trs[0] != "udp://from-file:80" {
		t.Errorf("trackers = %v, want [udp://from-file:80]", trs)
	}
}

func TestAddTrackers_MissingFileFallsBackToOriginalMagnet(t *testing.T) {
	got, err := AddTrackers(testMagnet, "/nonexistent/path/trackers.txt")
	if err != nil {
		t.Fatalf("AddTrackers: %v", err)
	}
	if got != testMagnet {
		t.Errorf("got %q, want unchanged magnet %q", got, testMagnet)
	}
}

func TestAddTrackers_ServerErrorFallsBackToOriginalMagnet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	got, err := AddTrackers(testMagnet, srv.URL)
	if err != nil {
		t.Fatalf("AddTrackers: %v", err)
	}
	if got != testMagnet {
		t.Errorf("got %q, want unchanged magnet %q", got, testMagnet)
	}
}

func TestAddTrackers_TimesOutRatherThanHanging(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never respond within the test
	}))
	// Deferred in this order so close(block) (LIFO: runs first) unblocks
	// the handler goroutine before srv.Close() (runs second) waits on it --
	// reversed, srv.Close() would deadlock waiting for a handler that can
	// only unblock after srv.Close() itself returns.
	defer srv.Close()
	defer close(block)

	original := httpClient.Timeout
	httpClient.Timeout = 200 * time.Millisecond
	defer func() { httpClient.Timeout = original }()

	start := time.Now()
	got, err := AddTrackers(testMagnet, srv.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("AddTrackers: %v", err)
	}
	if got != testMagnet {
		t.Errorf("got %q, want unchanged magnet %q", got, testMagnet)
	}
	if elapsed > 2*time.Second {
		t.Errorf("AddTrackers took %v, want it to respect the ~200ms client timeout", elapsed)
	}
}
