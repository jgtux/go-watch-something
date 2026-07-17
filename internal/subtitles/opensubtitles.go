package subtitles

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-watch-something/internal/utils"
)

// OpenSubtitles fetches subtitles directly from the OpenSubtitles REST
// API (https://opensubtitles.stoplight.io/docs/opensubtitles-api) -- no
// external binary required, unlike Subliminal. Used as a fallback when
// subliminal is unavailable or fails.
//
// Requires a free API key (OPENSUBTITLES_API_KEY) -- see README.
type OpenSubtitles struct {
	APIKey  string
	BaseURL string // defaults to the real API; overridable in tests
	Client  *http.Client
}

// NewOpenSubtitles reads the API key from OPENSUBTITLES_API_KEY.
func NewOpenSubtitles() OpenSubtitles {
	return OpenSubtitles{
		APIKey:  os.Getenv("OPENSUBTITLES_API_KEY"),
		BaseURL: "https://api.opensubtitles.com/api/v1",
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (o OpenSubtitles) Name() string { return "opensubtitles" }

func (o OpenSubtitles) Fetch(videoDir string, langs []string) error {
	if o.APIKey == "" {
		return fmt.Errorf("%w: OPENSUBTITLES_API_KEY not set", ErrNotConfigured)
	}

	videoName, err := findVideoName(videoDir)
	if err != nil {
		return err
	}
	query := guessTitle(videoName)

	fmt.Printf("Fetching subtitles via OpenSubtitles for %q...\n", query)

	fileID, err := o.search(query, langs)
	if err != nil {
		return fmt.Errorf("opensubtitles search: %w", err)
	}

	link, fileName, err := o.requestDownload(fileID)
	if err != nil {
		return fmt.Errorf("opensubtitles download request: %w", err)
	}

	if err := o.saveSubtitle(link, filepath.Join(videoDir, fileName)); err != nil {
		return fmt.Errorf("opensubtitles fetch subtitle: %w", err)
	}

	fmt.Println("Subtitles downloaded successfully via OpenSubtitles.")
	return nil
}

func (o OpenSubtitles) headers(req *http.Request) {
	req.Header.Set("Api-Key", o.APIKey)
	req.Header.Set("User-Agent", "go-watch-something")
	req.Header.Set("Content-Type", "application/json")
}

type searchResponse struct {
	Data []struct {
		Attributes struct {
			Files []struct {
				FileID int `json:"file_id"`
			} `json:"files"`
		} `json:"attributes"`
	} `json:"data"`
}

func (o OpenSubtitles) search(query string, langs []string) (int, error) {
	u := fmt.Sprintf("%s/subtitles?query=%s&languages=%s",
		o.BaseURL, url.QueryEscape(query), url.QueryEscape(strings.Join(langs, ",")))

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	o.headers(req)

	resp, err := o.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var parsed searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Attributes.Files) == 0 {
		return 0, fmt.Errorf("no subtitles found for %q", query)
	}
	return parsed.Data[0].Attributes.Files[0].FileID, nil
}

type downloadResponse struct {
	Link     string `json:"link"`
	FileName string `json:"file_name"`
}

func (o OpenSubtitles) requestDownload(fileID int) (link, fileName string, err error) {
	body, err := json.Marshal(map[string]int{"file_id": fileID})
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest(http.MethodPost, o.BaseURL+"/download", strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	o.headers(req)

	resp, err := o.Client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	var parsed downloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", "", err
	}
	if parsed.Link == "" {
		return "", "", fmt.Errorf("response had no download link")
	}
	if parsed.FileName == "" {
		parsed.FileName = "subtitle.srt"
	}
	return parsed.Link, parsed.FileName, nil
}

func (o OpenSubtitles) saveSubtitle(link, destPath string) error {
	resp, err := o.Client.Get(link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0o644)
}

// findVideoName returns the name of the first video file in dir.
func findVideoName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading video dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && utils.IsVideoFile(e.Name()) {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("no video file found in %s", dir)
}

// No trailing \b: "_" is a word character in regex terms, so a boundary
// check right after the marker would fail to match underscore-separated
// release names like "Movie_720p_BluRay" (verified -- an earlier version
// with \b silently skipped past "_720p" and only stripped "_BluRay").
// The leading [.\-_] delimiter is discrimination enough for this purpose.
var releaseNoise = regexp.MustCompile(`(?i)[.\-_](720p|1080p|2160p|4k|x264|x265|hevc|web[- ]?dl|webrip|bluray|brrip|dvdrip|hdtv|yify|rarbg|proper|repack).*$`)

// guessTitle turns a release-style filename into something closer to a
// searchable movie title. This is inherently approximate -- release-name
// parsing is a genuinely messy problem -- so it only strips the extension
// and common scene-release tags/quality markers, then normalizes
// separators to spaces.
func guessTitle(fileName string) string {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	name = releaseNoise.ReplaceAllString(name, "")
	name = strings.NewReplacer(".", " ", "_", " ").Replace(name)
	return strings.TrimSpace(name)
}
