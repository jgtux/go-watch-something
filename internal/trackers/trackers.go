// Package trackers appends a tracker list to a magnet link's "tr" query
// parameters, so a client with few/no trackers of its own still finds
// peers. The source is user-configurable (a URL or a local file), rather
// than a single hardcoded list.
package trackers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultSource is used when the caller doesn't specify one.
const DefaultSource = "https://raw.githubusercontent.com/ngosang/trackerslist/refs/heads/master/trackers_all_udp.txt"

var httpClient = &http.Client{Timeout: 10 * time.Second}

// AddTrackers appends every tracker line from source (a URL if it starts
// with "http://"/"https://", otherwise a local file path; DefaultSource if
// source is empty) to magnet's "tr" query parameters.
//
// Fetch/read failures fall back to returning the original magnet link
// unchanged rather than an error -- a missing tracker list shouldn't stop
// the download, since the magnet's own embedded trackers (if any) and DHT
// can still find peers.
func AddTrackers(magnet, source string) (string, error) {
	u, err := url.Parse(magnet)
	if err != nil {
		return "", err
	}

	if source == "" {
		source = DefaultSource
	}

	data, err := load(source)
	if err != nil {
		return magnet, nil
	}

	lines := strings.Split(string(data), "\n")
	q := u.Query()
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			q.Add("tr", line)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func load(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadURL(source)
	}
	return os.ReadFile(source)
}

func loadURL(source string) ([]byte, error) {
	resp, err := httpClient.Get(source)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trackers: fetching %s: unexpected status %s", source, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
