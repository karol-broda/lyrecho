# lyrecho

terminal-based synchronized lyrics viewer for linux music players that support mpris. displays real-time lyrics with dynamic color themes extracted from album artwork.

## features

- **real-time synced lyrics** - displays lyrics synchronized with playback
- **dynamic color themes** - extracts vibrant colors from album artwork
- **per-song sync adjustment** - fine-tune timing and save adjustments per track
- **persistent cache** - instant lyrics loading on subsequent plays
- **intelligent search** - case-insensitive with multiple fallback strategies
- **comprehensive cli** - manage cache, search lyrics, test player connections
- **smooth animations** - elegant transitions and effects

## quick reference

```bash
# start viewer
lyrecho

# cache management
lyrecho cache stats                    # show cache info
lyrecho cache list                     # list all cached songs
lyrecho cache clear                    # clear all cache

# player utilities
lyrecho player list                    # list mpris players
lyrecho player current                 # show what's playing

# lyrics tools
lyrecho lyrics preview "Artist" "Song" # preview lyrics
lyrecho lyrics fetch "Artist" "Song"   # pre-fetch to cache

# help
lyrecho --help                         # show all commands
lyrecho <command> --help               # command-specific help
```

## requirements

- linux system with d-bus
- music player with mpris support (e.g., spotify, vlc, mpv, mpd)
- go 1.21 or later (for building)

## installation

```bash
git clone https://github.com/karol-broda/lyrecho
cd lyrecho
go build -o lyrecho ./cmd/lyrecho
sudo mv lyrecho /usr/local/bin/  # optional: install system-wide
```

## shell completion

lyrecho supports auto-completion for bash, zsh, fish, and powershell:

```bash
# bash
lyrecho completion bash > /etc/bash_completion.d/lyrecho
# or for current user only
lyrecho completion bash > ~/.local/share/bash-completion/completions/lyrecho

# zsh
lyrecho completion zsh > "${fpath[1]}/_lyrecho"

# fish
lyrecho completion fish > ~/.config/fish/completions/lyrecho.fish

# powershell
lyrecho completion powershell > lyrecho.ps1
```

after installing completion, restart your shell or source the completion file.

## usage

### interactive viewer (default)

start the tui viewer while music is playing:

```bash
lyrecho
# or explicitly
lyrecho run
```

**keyboard controls:**

| key | action |
|-----|--------|
| `↑` / `k` / `+` / `=` | increase sync offset by 0.1s |
| `↓` / `j` / `-` | decrease sync offset by 0.1s |
| `→` / `l` | increase sync offset by 0.5s |
| `←` / `h` | decrease sync offset by 0.5s |
| `0` | reset sync offset to 0 |
| `q` / `ctrl+c` / `esc` | quit |

**note:** sync offset adjustments are automatically saved per-song in the cache.

### cache management

manage your cached lyrics:

```bash
# show cache statistics
lyrecho cache stats

# list all cached songs with sync offsets
lyrecho cache list
lyrecho cache list --sort=artist  # sort by artist
lyrecho cache list --sort=title   # sort by title

# show details for specific song
lyrecho cache show "Chappell Roan" "HOT TO GO!"

# remove specific song
lyrecho cache delete "Artist" "Title"

# remove expired entries
lyrecho cache prune

# clear entire cache
lyrecho cache clear
lyrecho cache clear --confirm  # skip confirmation
```

### player utilities

discover and test mpris players:

```bash
# list all available mpris players
lyrecho player list

# test connection to configured player
lyrecho player test

# test specific player
lyrecho player test --service=org.mpris.MediaPlayer2.vlc

# show currently playing track
lyrecho player current
```

### lyrics search and preview

search and preview lyrics without starting the viewer:

```bash
# search for lyrics on lrclib
lyrecho lyrics search "Chappell Roan" "HOT TO GO!"

# pre-fetch lyrics to cache
lyrecho lyrics fetch "Artist" "Title"

# preview lyrics in terminal with timestamps
lyrecho lyrics preview "Chappell Roan" "HOT TO GO!"
```

## configuration

### environment variables

- `MPRIS_SERVICE` - mpris service name (default: `org.mpris.MediaPlayer2.spotify`)
- `LRCLIB_GET_URL` - lrclib api endpoint (default: `https://lrclib.net/api/get`)
- `SYNC_OFFSET` - global initial sync offset in seconds (default: `0`)
- `HIDE_HEADER` - hide header section (default: `false`)
- `LYRECHO_USE_KITTY_GRAPHICS` - opt-in to use kitty graphics protocol for album art display instead of half-block rendering (values: `1`/`true`/`yes`/`on` to enable; default is half-block rendering)

**example: enable kitty graphics protocol for high-quality album art:**

```bash
# test kitty graphics protocol (kitty, ghostty, and compatible terminals)
LYRECHO_USE_KITTY_GRAPHICS=1 lyrecho

# or export for persistent use
export LYRECHO_USE_KITTY_GRAPHICS=1
lyrecho
```

### command-line flags

```bash
# use different music player
lyrecho -m org.mpris.MediaPlayer2.vlc

# start with custom offset
lyrecho -s 0.5

# hide header
lyrecho -H

# disable cache (always fetch fresh)
lyrecho --no-cache

# custom lrclib url
lyrecho --lrclib-url https://custom.lrclib.url/api/get
```

### finding your mpris service name

use the player discovery command:

```bash
lyrecho player list
```

or manually with dbus:

```bash
dbus-send --session --dest=org.freedesktop.DBus --type=method_call \
  --print-reply /org/freedesktop/DBus org.freedesktop.DBus.ListNames | \
  grep mpris
```

## how it works

1. connects to your music player via d-bus mpris interface
2. retrieves currently playing track information (title, artist, album, artwork)
3. queries lrclib.net api for synchronized lyrics with intelligent fallback strategies:
   - tries normalized names with album/duration
   - strips version info (remixes, live versions, etc.)
   - attempts uppercase, lowercase, and title case variations
   - ensures high success rate regardless of how the artist/title is formatted
4. analyzes album artwork to extract vibrant colors for theming using hsl color space
5. polls playback position and displays the appropriate lyric line with smooth transitions
6. automatically updates when track changes
7. caches lyrics and per-song sync offsets locally for instant loading

## cache details

- **location:** `~/.cache/lyric-shower/lyrics/` (or `$XDG_CACHE_HOME/lyric-shower/lyrics/`)
- **format:** binary (gob encoding) with .bin extension
- **ttl:** 30 days (automatically pruned)
- **stored data:** lyrics (synced + plain), track metadata, per-song sync offset

## limitations

- requires synced lyrics to be available on lrclib.net
- only works on linux systems with d-bus
- music player must support mpris interface
- cannot pre-fetch next song in queue (mpris does not expose queue information)
  - first play of a song may have a brief delay while fetching lyrics
  - subsequent plays are instant thanks to local caching

## troubleshooting

**no mpris players found:**
- ensure your music player is running
- check if it supports mpris (spotify, vlc, mpv, mpd all do)
- run `lyrecho player list` to see available players

**lyrics not syncing properly:**
- use `↑↓←→` keys to adjust timing in real-time
- adjustments are saved automatically per song
- try `0` to reset if you over-adjust

**lyrics not found:**
- not all songs have synced lyrics on lrclib.net
- the tool automatically tries 8+ different search variations (case, formatting, etc.)
- search manually at https://lrclib.net to verify availability
- consider contributing lyrics to lrclib if missing

**special characters in song titles:**
- use single quotes to prevent shell interpretation: `'HOT TO GO!'`
- or escape special characters: `"HOT TO GO\!"`
- the `!` character can cause issues in bash due to history expansion
- tip: use `set +H` to temporarily disable history expansion
- note: lyrecho searches are case-insensitive, so `"surf curse"` will find `"SURF CURSE"`

**fuzzy matching:**
- if you get "song not found", check the suggestions
- the cli will suggest similar songs from your cache
- example: `"HOT TO GO"` → suggests `"HOT TO GO!"`
- works for both `cache show/delete` and `lyrics preview` commands

**colors look wrong:**
- check if your terminal supports true color
- some terminal emulators may have limited color support
- default palette will be used if artwork extraction fails

## examples

```bash
# start viewer with vlc
lyrecho -m org.mpris.MediaPlayer2.vlc

# check what's playing without starting viewer
lyrecho player current

# preview lyrics for a song
lyrecho lyrics preview "TV Girl" "Lovers Rock"

# pre-fetch lyrics for your playlist
lyrecho lyrics fetch "Gorillaz" "Feel Good Inc."
lyrecho lyrics fetch "Daft Punk" "Get Lucky"

# check cache and clean up
lyrecho cache stats
lyrecho cache prune

# view all cached songs sorted by artist
lyrecho cache list --sort=artist

# show sync offset for specific song
lyrecho cache show "Chappell Roan" "Good Luck, Babe!"
```

## development

### running without building

when developing, you can run lyrecho directly with `go run`:

```bash
# run all files in the cmd/lyrecho directory
go run ./cmd/lyrecho [args]

# examples:
go run ./cmd/lyrecho player list
go run ./cmd/lyrecho lyrics preview "Artist" "Title"
go run ./cmd/lyrecho cache stats
go run ./cmd/lyrecho  # start the viewer
```

**note:** you must use `./cmd/lyrecho` (not `cmd/lyrecho/main.go`) so that go compiles all files in the package, not just main.go.

### building

```bash
go build -o lyrecho ./cmd/lyrecho
```

### testing

```bash
go test ./...
```

## contributing

contributions are welcome! feel free to open issues or submit pull requests.

## license

mit license - see license file for details

## acknowledgments

- lyrics data from [lrclib.net](https://lrclib.net)
- uses [bubble tea](https://github.com/charmbracelet/bubbletea) for tui
- color extraction with [prominentcolor](https://github.com/EdlinOrg/prominentcolor)