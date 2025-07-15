package main

import (
	// "bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"encoding/json"
	"github.com/anacrolix/torrent"
)

func main() {
	serveBufAtFlag := flag.Float64("serve_at", 0.02, "Float of range [0,1].")
	portFlag := flag.Uint("port", 8080, "Int of range [0, 65535].")
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
	if magnet == "" || !isValidMagnetLink(magnet) {
		log.Fatal("Valid magnet link is required.")
	}

	magnet, err := addUDPTrackers(magnet)
	if err != nil {
		log.Fatal(err)
	}

	clientConfig := torrent.NewDefaultClientConfig()
	tmpDir, err := os.MkdirTemp("", "torrent-stream-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanTmpDir(tmpDir)

	clientConfig.DataDir = tmpDir
	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	t, err := client.AddMagnet(magnet)
	if err != nil {
		log.Fatal(err)
	}

	clearScreen()
	fmt.Println("Fetching metadata...")

	select {
	case <-t.GotInfo():
		fmt.Println("Metadata fetched!")
	case <-time.After(120 * time.Second):
		log.Fatal("Timeout while fetching metadata.")
	}

	time.Sleep(1 * time.Second)

	var largestFile *torrent.File
	for _, f := range t.Files() {
		if isVideoFile(f.Path()) && (largestFile == nil || f.Length() > largestFile.Length()) {
			largestFile = f
		}
	}
	if largestFile == nil {
		log.Fatal("No video file found in torrent.")
	}

	fmt.Printf("Selected file: %s (%.2f MB)", largestFile.Path(), float64(largestFile.Length())/1024/1024)

	if useSubliminal {
		go func() {
			subLangList := strings.Split(subLangs, ",")
			subPath := filepath.Join(tmpDir, largestFile.Path())
			err := getSubsFromSubliminal(subPath, subLangList)
			if err != nil {
				log.Printf("Subtitle download failed: %v", err)
			}
			// else {
			// 	err := renameAllSubtitlesToSequential(filepath.Dir(subPath))
			// 	if err != nil {
			// 		log.Printf("Subtitle rename failed: %v", err)
			// 	}
			// }
		}()
	}

	t.DownloadAll()
	largestFile.Download()

	bufferSize := int64(float64(largestFile.Length()) * *serveBufAtFlag)

	go func() {
		for largestFile.BytesCompleted() < bufferSize {
			time.Sleep(500 * time.Millisecond)
			stats := t.Stats()
			progress := float64(t.BytesCompleted()) / float64(t.Length()) * 100
			fmt.Printf("\rPeers: %d | Seeders: %d | Progress: %.2f%% | Buffer: %d / %d",
				stats.ActivePeers, stats.ConnectedSeeders, progress,
				largestFile.BytesCompleted(), bufferSize)
		}
		fmt.Println("\n\nBuffering complete, ready to serve!")
		fmt.Printf("Movie at http://localhost:%d/movie\n", *portFlag)
		if useSubliminal {
			fmt.Printf("Subs at http://localhost:%d/subs/\n", *portFlag)
		}
	}()

	// Serve o vídeo principal em /movie
	http.HandleFunc("/movie", func(w http.ResponseWriter, r *http.Request) {
		if largestFile.BytesCompleted() < bufferSize {
			http.Error(w, "Buffering... please wait", http.StatusServiceUnavailable)
			return
		}
		modTime := time.Now()
		http.ServeContent(w, r, filepath.Base(largestFile.Path()), modTime, largestFile.NewReader())
	})

	if useSubliminal {
		// Serve a lista e os arquivos de legendas em /subs e /subs/{file}
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

				// Responder JSON com a lista de legendas
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(subs)
				return
			}

			// Servir arquivo .srt específico
			fullSubPath := filepath.Join(subsDir, subPath)
			http.ServeFile(w, r, fullSubPath)
		})
	}

	go func() {
		fmt.Printf("Starting stream server on http://localhost:%d", *portFlag)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), nil); err != nil {
			log.Fatal(err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	fmt.Println("\n\nInterrupt received. Exiting.")
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

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func cleanTmpDir(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("Failed to remove tmp dir: %v", err)
	}
}

func getSubsFromSubliminal(file string, langs []string) error {
	fmt.Println("\nFetching subtitles from subliminal...")

	fmt.Println(langs)
	fmt.Println(file)
	args := []string{"download"}
	for _, lang := range langs {
		args = append(args, "-l", lang)
	}
	
	args = append(args, file)

	log.Printf("Running subliminal with args: %#v\n", args)

	cmd := exec.Command("subliminal", args...)
	cmd.Stdout = io.Discard 
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Subtitles download failed: %w", err)
	}
	return nil 
}



// func renameAllSubtitlesToSequential(fpath string) error {
// 	files, err := os.ReadDir(fpath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read dir: %w", err)
// 	}

// 	counter := 1
// 	for _, f := range files {
// 		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".srt") {
// 			oldPath := filepath.Join(fpath, f.Name())
// 			newName := fmt.Sprintf("sub%d.srt", counter)
// 			newPath := filepath.Join(fpath, newName)

// 			err := os.Rename(oldPath, newPath)
// 			if err != nil {
// 				return fmt.Errorf("failed to rename %s to %s: %w", f.Name(), newName, err)
// 			}

// 			log.Printf("Renamed subtitle: %s → %s", f.Name(), newName)
// 			counter++
// 		}
// 	}

// 	return nil
// }
