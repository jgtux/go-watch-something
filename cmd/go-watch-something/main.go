package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "path/filepath"

    "github.com/jgtux/go-watch-something/internal/streamer"
    "github.com/jgtux/go-watch-something/internal/subtitles"
    "github.com/jgtux/go-watch-something/internal/utils"
)

func main() {
    serveBufAtFlag := flag.Float64("serve_at", 0.02, "Float of range [0,1].")
    portFlag := flag.Uint("port", 8080, "Port to serve content on.")
    var useSubliminal bool
    flag.BoolVar(&useSubliminal, "use_subliminal", false, "Use subliminal to fetch subtitles.")
    var subLangs string
    flag.StringVar(&subLangs, "subliminal_langs", "en", "Comma-separated subtitle langs: en,pt-BR,...")
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

    magnet, err := streamer.AddUDPTrackers(magnet)
    if err != nil {
        log.Fatal(err)
    }

    tmpDir, client, t := streamer.SetupTorrentClient(magnet)
    defer streamer.CleanUp(tmpDir, client)

    largestFile := streamer.SelectLargestVideo(t)
    fmt.Printf("Selected file: %s\n", largestFile.Path())

    if useSubliminal {
        langs := utils.ParseLangs(subLangs)
	videoPath := filepath.Join(tmpDir, largestFile.Path())
	videoDir := filepath.Dir(videoPath)
	if err := subtitles.Fetch(videoDir, langs); err != nil {
            log.Printf("Failed to fetch subtitles: %v\nContinuing without subtitles.", err)
            useSubliminal = false
        }
    }

    streamer.StartDownload(t, largestFile)

    // Serve HTTP endpoints
    streamer.StartHTTPServer(*portFlag, largestFile, tmpDir, useSubliminal)

    // Handle interrupts
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
    <-sigs
    fmt.Println("\nInterrupt received. Exiting.")
}
