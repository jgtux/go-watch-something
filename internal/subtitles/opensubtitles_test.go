package subtitles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSubtitles_NotConfiguredWithoutAPIKey(t *testing.T) {
	o := OpenSubtitles{APIKey: ""}
	err := o.Fetch(t.TempDir(), []string{"en"})
	if err == nil {
		t.Fatal("Fetch with no API key = nil, want error")
	}
	if !isNotConfigured(err) {
		t.Errorf("Fetch with no API key = %v, want it to wrap ErrNotConfigured", err)
	}
}

func isNotConfigured(err error) bool {
	for err != nil {
		if err == ErrNotConfigured {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestOpenSubtitles_FullSearchDownloadFlow(t *testing.T) {
	// The final fetch of the actual .srt bytes deliberately doesn't send
	// Api-Key (saveSubtitle uses a plain Client.Get) -- the real API
	// returns that download link on a different host (dl.opensubtitles.org
	// vs api.opensubtitles.com), so sending the key there would leak it to
	// an unrelated domain. Capture the key per-endpoint rather than in one
	// shared variable that the unauthenticated final request would clobber.
	var searchAPIKey, downloadAPIKey string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/subtitles":
			searchAPIKey = r.Header.Get("Api-Key")
			if got := r.URL.Query().Get("query"); got != "Some Movie 2024" {
				t.Errorf("search query = %q, want %q", got, "Some Movie 2024")
			}
			json.NewEncoder(w).Encode(searchResponse{
				Data: []struct {
					Attributes struct {
						Files []struct {
							FileID int `json:"file_id"`
						} `json:"files"`
					} `json:"attributes"`
				}{{
					Attributes: struct {
						Files []struct {
							FileID int `json:"file_id"`
						} `json:"files"`
					}{Files: []struct {
						FileID int `json:"file_id"`
					}{{FileID: 42}}},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/download":
			downloadAPIKey = r.Header.Get("Api-Key")
			var body map[string]int
			json.NewDecoder(r.Body).Decode(&body)
			if body["file_id"] != 42 {
				t.Errorf("download file_id = %d, want 42", body["file_id"])
			}
			json.NewEncoder(w).Encode(downloadResponse{
				Link:     srv.URL + "/actual-file",
				FileName: "Some.Movie.en.srt",
			})
		case r.URL.Path == "/actual-file":
			w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	videoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(videoDir, "Some.Movie.2024.1080p.WEB-DL.x264.mkv"), []byte("fake video"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	o := OpenSubtitles{APIKey: "test-key", BaseURL: srv.URL, Client: http.DefaultClient}
	if err := o.Fetch(videoDir, []string{"en"}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if searchAPIKey != "test-key" {
		t.Errorf("search Api-Key header = %q, want %q", searchAPIKey, "test-key")
	}
	if downloadAPIKey != "test-key" {
		t.Errorf("download Api-Key header = %q, want %q", downloadAPIKey, "test-key")
	}

	srtPath := filepath.Join(videoDir, "Some.Movie.en.srt")
	data, err := os.ReadFile(srtPath)
	if err != nil {
		t.Fatalf("expected subtitle file at %s: %v", srtPath, err)
	}
	if len(data) == 0 {
		t.Errorf("subtitle file is empty")
	}
}

func TestOpenSubtitles_NoResultsIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResponse{})
	}))
	defer srv.Close()

	videoDir := t.TempDir()
	os.WriteFile(filepath.Join(videoDir, "movie.mkv"), []byte("x"), 0o644)

	o := OpenSubtitles{APIKey: "test-key", BaseURL: srv.URL, Client: http.DefaultClient}
	if err := o.Fetch(videoDir, []string{"en"}); err == nil {
		t.Fatal("Fetch with no search results = nil, want error")
	}
}

func TestGuessTitle(t *testing.T) {
	cases := map[string]string{
		"Some.Movie.2024.1080p.WEB-DL.x264.mkv": "Some Movie 2024",
		"Another_Movie_720p_BluRay.mp4":         "Another Movie",
		"plain name.mkv":                        "plain name",
	}
	for input, want := range cases {
		if got := guessTitle(input); got != want {
			t.Errorf("guessTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFindVideoName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "movie.mkv"), []byte("x"), 0o644)

	got, err := findVideoName(dir)
	if err != nil {
		t.Fatalf("findVideoName: %v", err)
	}
	if got != "movie.mkv" {
		t.Errorf("findVideoName = %q, want %q", got, "movie.mkv")
	}
}

func TestFindVideoName_NoVideoFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644)

	if _, err := findVideoName(dir); err == nil {
		t.Fatal("findVideoName with no video file = nil error, want error")
	}
}
