package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go-watch-something/internal/player"
	"go-watch-something/internal/streamer"
	"go-watch-something/internal/subtitles"
	"go-watch-something/internal/trackers"
	"go-watch-something/internal/utils"
)

func main() {
	serveBufAtFlag := flag.Float64("serve_at", 0.02, "Float of range [0,1].")
	portFlag := flag.Uint("port", 8080, "Port to serve content on.")
	hostFlag := flag.String("host", "127.0.0.1", "Host to bind the server to. Use 0.0.0.0 to allow LAN access.")
	var inMemory bool
	flag.BoolVar(&inMemory, "in-memory", false, "Keep torrent piece data in memory instead of writing it to a temp dir.")
	var trackersSource string
	flag.StringVar(&trackersSource, "trackers", "", "URL or local file path for the tracker list. Empty uses a built-in default.")
	var autoplay bool
	flag.BoolVar(&autoplay, "autoplay", false, "Launch a video player automatically once buffering completes.")
	var playerOverride string
	flag.StringVar(&playerOverride, "player", "", "Command to launch for -autoplay. Empty auto-detects xdg-open, then mpv, then vlc.")
	var wantSubs bool
	flag.BoolVar(&wantSubs, "subs", false, "Fetch subtitles (tries subliminal, then the OpenSubtitles API).")
	var subLangs string
	flag.StringVar(&subLangs, "sub-langs", "en", "Comma-separated subtitle langs: en,pt-BR,...")
	var magnet string
	flag.StringVar(&magnet, "magnet", "", "Magnet link to stream.")
	flag.Parse()

	if *serveBufAtFlag < 0 || *serveBufAtFlag > 1 {
		log.Fatal("Flag serve_at must be in range [0,1].")
	}
	if *portFlag > 65535 {
		log.Fatal("Flag port must be in range [0, 65535].")
	}
	if magnet == "" || !utils.IsValidMagnetLink(magnet) {
		log.Fatal("Valid magnet link is required.")
	}

	magnet, err := trackers.AddTrackers(magnet, trackersSource)
	if err != nil {
		log.Fatal(err)
	}

	tmpDir, client, t := streamer.SetupTorrentClient(magnet, inMemory)
	defer streamer.CleanUp(tmpDir, client)

	largestFile := streamer.SelectLargestVideo(t)
	fmt.Printf("Selected file: %s\n", largestFile.Path())

	if wantSubs {
		langs := utils.ParseLangs(subLangs)
		videoPath := filepath.Join(tmpDir, largestFile.Path())
		videoDir := filepath.Dir(videoPath)
		providers := []subtitles.Provider{subtitles.Subliminal{}, subtitles.NewOpenSubtitles()}
		if err := subtitles.FetchWithFallback(providers, videoDir, langs); err != nil {
			log.Printf("Failed to fetch subtitles: %v\nContinuing without subtitles.", err)
			wantSubs = false
		}
	}

	streamer.StartDownload(t, largestFile)

	// Serve HTTP endpoints
	streamer.StartHTTPServer(*hostFlag, *portFlag, largestFile, tmpDir, wantSubs)

	if autoplay {
		streamURL := fmt.Sprintf("http://%s:%d/movie", *hostFlag, *portFlag)
		if err := player.Launch(streamURL, playerOverride); err != nil {
			log.Printf("Autoplay failed: %v\nOpen the URL above manually.", err)
		}
	}

	// Handle interrupts
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	fmt.Println("\nInterrupt received. Exiting.")
}
