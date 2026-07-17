# go-watch-Something 

Stream movies directly from torrents and watch them, built with Go. Work in progress.

---

## Features

- Stream torrent data on-the-fly, served over local HTTP (bound to `127.0.0.1` by default)
- Simple CLI interface with magnet link input
- Configurable tracker list (a URL or a local file, not a single hardcoded source)
- Optional autoplay -- launches `xdg-open`, then falls back through `mpv`/`vlc`
- Optional in-memory mode -- keeps torrent piece data in RAM instead of writing it to a temp dir
- Subtitles with fallback: tries `subliminal` first, then the OpenSubtitles API directly if that's unavailable
- Configurable via command-line flags

---

## Requirements

- Go 1.23 or higher
- Internet connection to fetch torrent data
- `subliminal` (optional, for subtitles) -- or an `OPENSUBTITLES_API_KEY` (see below)
- For `-autoplay`: `xdg-open` (Linux/BSD), `mpv`, or `vlc` on `PATH` -- or pass `-player` explicitly

---

## Installation

Clone the repository and build:

```bash
$ git clone https://github.com/jgtux/go-watch-something.git
$ cd go-watch-something
$ make install
```

## Usage

```bash
$ go-watch-something -h
$ # Basic
$ go-watch-something -magnet="<magnet>"
$ # Subtitles, autoplay, in-memory, custom tracker list
$ go-watch-something -magnet="<magnet>" -subs -autoplay -in-memory -trackers=/path/to/trackers.txt
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-magnet` | (required) | Magnet link to stream |
| `-port` | `8080` | Port to serve content on |
| `-host` | `127.0.0.1` | Host to bind the server to. Use `0.0.0.0` to allow LAN access |
| `-in-memory` | `false` | Keep torrent piece data in memory instead of a temp dir |
| `-trackers` | *(built-in list)* | URL or local file path for the tracker list |
| `-autoplay` | `false` | Launch a video player automatically once buffering completes |
| `-player` | *(auto-detect)* | Force a specific player command for `-autoplay` |
| `-subs` | `false` | Fetch subtitles (subliminal, then OpenSubtitles) |
| `-sub-langs` | `en` | Comma-separated subtitle languages |
| `-serve_at` | `0.02` | Fraction of the file to buffer before serving starts |

### Subtitles

Two providers are tried in order:

1. **subliminal** -- requires the [`subliminal`](https://github.com/Diaoul/subliminal) CLI installed separately.
2. **OpenSubtitles API** -- requires a free API key from [opensubtitles.com](https://www.opensubtitles.com/), set as `OPENSUBTITLES_API_KEY`. Used automatically if `subliminal` is missing or fails; skipped silently if the env var isn't set.

Movie title matching for the OpenSubtitles fallback is best-effort: it strips the file extension and common release tags (`1080p`, `x264`, `WEB-DL`, ...) from the torrent's video filename and searches on what's left. Release-name parsing is inherently approximate.
