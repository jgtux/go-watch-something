package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"os/signal"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"github.com/anacrolix/torrent"
)

func main() {	
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <magnet_link>")
	}
	magnet := os.Args[1]

	if !isValidMagnetLink(magnet) {
		log.Fatal("Invalid magnet.")
	}

	magnetWithExtraUDPTr, err := addUDPTrackers(magnet)
	if err != nil {
		log.Fatal(err)
	}
	
	clientConfig := torrent.NewDefaultClientConfig()
	tmpDir, err := os.MkdirTemp("", "torrent-stream-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	clientConfig.DataDir = tmpDir
	clientConfig.NoUpload = false
	clientConfig.Seed = false
	clientConfig.Debug = false

	defer func() {
		if err := cleanTmpDir(clientConfig.DataDir); err != nil {
			log.Printf("Error cleaning tmp dir: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	t, err := client.AddMagnet(magnetWithExtraUDPTr)
	if err != nil {
		log.Fatal(err)
	}

	clearScreen()
	log.Println("Fetching metadata...")

	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	done := false
	for !done {
		select {
		case <-t.GotInfo():
			log.Println("Metadata fully fetched!")
			done = true
		case <-timeout:
			log.Println("Timeout fetching metadata.")
			return
		case <-ticker.C:
			clearScreen()
			log.Println("Waiting for metadata...")
		}
	}

	time.Sleep(1 * time.Second) // Small delay for stability

	var largestFile *torrent.File 

	for _, f := range t.Files() {
		if isVideoFile(f.Path()) && (largestFile == nil || f.Length() > largestFile.Length()) {
			largestFile = f
		}
	}

	if largestFile == nil {
		log.Fatal("Unsupported or no video found in torrent.")
}

	log.Printf("Selected file: %s (%.2f MB)\n", largestFile.Path(), float64(largestFile.Length())/1024/1024)

	t.DownloadAll() // prevent locking
	largestFile.Download()
	
	bufferSize := int64(float64(largestFile.Length()) * 0.02) // x bytes * 0.02 - buffer based in file length

	// Pre-buffering goroutine
	go func() {
		for largestFile.BytesCompleted() < bufferSize {
			time.Sleep(500 * time.Millisecond)
			stats := t.Stats()

			progress := float64(t.BytesCompleted()) / float64(t.Length()) * 100
			log.Printf("Peers connected: %d", stats.ActivePeers)
			log.Printf("Seeders: %d", stats.ConnectedSeeders)
			log.Printf("Progress: %.2f%% (%d / %d bytes)", progress, t.BytesCompleted(), t.Length())
			log.Printf("Buffering... %d / %d bytes\n", largestFile.BytesCompleted(), bufferSize)
		}
		clearScreen()
		log.Println("Buffering complete, ready to serve")
		log.Println("Stream server on http://localhost:8080")

	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if largestFile.BytesCompleted() < bufferSize {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Buffering... please wait"))
			return
		}

		modTime := time.Now()
		http.ServeContent(w, r, filepath.Base(largestFile.Path()), modTime, largestFile.NewReader())
	})

	go func() {
		log.Println("Starting stream server on http://localhost:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}()

	<-sigs
	log.Println("Signal received, exiting...")
}

func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov"
}

func isValidMagnetLink(magnet string) bool {
	if !strings.HasPrefix(magnet, "magnet:?") {
		return false
	}
	u, err := url.Parse(magnet)
	if err != nil {
		return false
	}
	values, _ := url.ParseQuery(u.RawQuery)
	xt := values.Get("xt")
	if xt == "" || !strings.HasPrefix(xt, "urn:btih:") {
		return false
	}
	hash := xt[len("urn:btih:"):]
	isHex, _ := regexp.MatchString("^[a-fA-F0-9]{40}$", hash)
	isBase32, _ := regexp.MatchString("^[A-Z2-7]{32}$", strings.ToUpper(hash))
	return isHex || isBase32
}

func addUDPTrackers(magnet string) (string, error) {
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
		log.Print("Could not fetch additional trackers for streaming, continuing...")
		return magnet, nil
	}
	rawTrackers, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print("Could not read tracker list, continuing...")
		return magnet, nil
	}

	var udpTrackersArray []string
	lines := bytes.Split(rawTrackers, []byte("\n"))
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		udpTrackersArray = append(udpTrackersArray, trimmed)
	}

	q := u.Query()
	for _, tr := range udpTrackersArray {
		exists := false
		for _, existing := range q["tr"] {
			if existing == tr {
				exists = true
				break
			}
		}
		if !exists {
			q.Add("tr", tr)
		}
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

func cleanTmpDir(dir string) error {
	err := os.RemoveAll(dir)
	if err != nil {
		log.Printf("️ Failed to remove %s: %v", dir, err)
	}
	return nil
}
