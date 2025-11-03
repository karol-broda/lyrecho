# lyrecho

terminal-based synchronized lyrics viewer for linux music players that support mpris. displays real-time lyrics with dynamic color themes extracted from album artwork.

## requirements

- linux system with d-bus
- music player with mpris support (e.g., spotify, vlc, mpv)
- go 1.21 or later (for building)

## installation

```bash
git clone https://github.com/yourusername/lyrecho
cd lyrecho
go build
```

## configuration

configure using environment variables:

- `MPRIS_SERVICE` - mpris service name (default: `org.mpris.MediaPlayer2.spotify`)
- `LRCLIB_GET_URL` - lrclib api endpoint (default: `https://lrclib.net/api/get`)
- `SYNC_OFFSET` - initial sync offset in seconds (default: `0`)

### examples

```bash
# use with different music player
MPRIS_SERVICE=org.mpris.MediaPlayer2.vlc ./lyrecho

# start with custom offset
SYNC_OFFSET=0.5 ./lyrecho
```

### finding your mpris service name

```bash
dbus-send --session --dest=org.freedesktop.DBus --type=method_call \
  --print-reply /org/freedesktop/DBus org.freedesktop.DBus.ListNames | \
  grep mpris
```

## usage

simply run the binary while music is playing:

```bash
./lyrecho
```

### keyboard controls

| key | action |
|-----|--------|
| `↑` / `k` / `+` / `=` | increase sync offset by 0.1s |
| `↓` / `j` / `-` | decrease sync offset by 0.1s |
| `→` / `l` | increase sync offset by 0.5s |
| `←` / `h` | decrease sync offset by 0.5s |
| `0` | reset sync offset to 0 |
| `q` / `ctrl+c` / `esc` | quit |

## how it works

1. connects to your music player via d-bus mpris interface
2. retrieves currently playing track information (title, artist, album, artwork)
3. queries lrclib.net api for synchronized lyrics
4. analyzes album artwork to extract vibrant colors for theming
5. polls playback position and displays the appropriate lyric line
6. automatically updates when track changes

## limitations

- requires synced lyrics to be available on lrclib.net
- only works on linux systems with d-bus
- music player must support mpris interface
