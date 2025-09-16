package streamer

import (
    "fmt"
    "log"
    "os"
    "time"
    "net/http"
    "path/filepath"
    "strings"
    "encoding/json"
    "net/url"
    "io"


    "github.com/anacrolix/torrent"
    "github.com/jgtux/go-watch-something/internal/utils"
)

// SetupTorrentClient creates client, tmpDir, adds magnet link, waits metadata
func SetupTorrentClient(magnet string) (tmpDir string, client *torrent.Client, t *torrent.Torrent) {
    tmpDir, err := os.MkdirTemp("", "torrent-stream-*")
    if err != nil {
        log.Fatalf("Failed to create temp dir: %v", err)
    }

    clientConfig := torrent.NewDefaultClientConfig()
    clientConfig.DataDir = tmpDir
    client, err = torrent.NewClient(clientConfig)
    if err != nil {
        log.Fatal(err)
    }

    t, err = client.AddMagnet(magnet)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Fetching metadata...")
    select {
    case <-t.GotInfo():
        fmt.Println("Metadata fetched!")
    case <-time.After(120 * time.Second):
        log.Fatal("Timeout while fetching metadata.")
    }

    return tmpDir, client, t
}

func SelectLargestVideo(t *torrent.Torrent) *torrent.File {
    var largestFile *torrent.File
    for _, f := range t.Files() {
        if utils.IsVideoFile(f.Path()) && (largestFile == nil || f.Length() > largestFile.Length()) {
            largestFile = f
        }
    }
    if largestFile == nil {
        log.Fatal("No video file found in torrent.")
    }
    return largestFile
}

func StartDownload(t *torrent.Torrent, largestFile *torrent.File) {
	t.DownloadAll()
	largestFile.Download()

	// Example buffering logic: wait until 2% of file is downloaded
	bufferSize := int64(float64(largestFile.Length()) * 0.02)
	fmt.Println("Buffering...")

	for largestFile.BytesCompleted() < bufferSize {
		time.Sleep(500 * time.Millisecond)
		stats := t.Stats()
		progress := float64(t.BytesCompleted()) / float64(t.Length()) * 100
		fmt.Printf("\rPeers: %d | Seeders: %d | Progress: %.2f%% | Buffer: %d / %d",
			stats.ActivePeers, stats.ConnectedSeeders, progress,
			largestFile.BytesCompleted(), bufferSize)
	}
	fmt.Println("\nBuffering complete!")
}

// CleanUp closes the torrent client and removes the temporary directory
func CleanUp(tmpDir string, client *torrent.Client) {
	client.Close()
	if err := os.RemoveAll(tmpDir); err != nil {
		log.Printf("Failed to remove tmp dir: %v", err)
	}
}

// StartHTTPServer serves the video and optional subtitles over HTTP
func StartHTTPServer(port uint, largestFile *torrent.File, tmpDir string, useSubs bool) {
	// Serve /movie
	http.HandleFunc("/movie", func(w http.ResponseWriter, r *http.Request) {
		modTime := time.Now()
		http.ServeContent(w, r, filepath.Base(largestFile.Path()), modTime, largestFile.NewReader())
	})

	if useSubs {
		subsDir := filepath.Join(tmpDir, filepath.Dir(largestFile.Path()))
		http.HandleFunc("/subs/", func(w http.ResponseWriter, r *http.Request) {
			subPath := strings.TrimPrefix(r.URL.Path, "/subs/")
			if subPath == "" || subPath == "/" {
				files, err := os.ReadDir(subsDir)
				if err != nil {
					http.Error(w, "Failed to list subtitles", http.StatusInternalServerError)
					return
				}
				var subs []string
				for _, f := range files {
					if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
						subs = append(subs, f.Name())
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(subs)
				return
			}

			// Serve a specific .srt file
			fullSubPath := filepath.Join(subsDir, subPath)
			http.ServeFile(w, r, fullSubPath)
		})
	}

	// Start server
	go func() {
		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("Server running at http://localhost%s/movie\n", addr)
		if useSubs {
			fmt.Printf("Subtitles at http://localhost%s/subs/\n", addr)
		}
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal(err)
		}
	}()
}

func AddUDPTrackers(magnet string) (string, error) {
	u, err := url.Parse(magnet)
	if err != nil {
		return "", err
	}
	resp, err := http.Get("https://raw.githubusercontent.com/ngosang/trackerslist/refs/heads/master/trackers_all_udp.txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return magnet, nil
	}
	data, err := io.ReadAll(resp.Body)
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

