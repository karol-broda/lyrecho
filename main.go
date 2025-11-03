package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/EdlinOrg/prominentcolor"
	"github.com/godbus/dbus/v5"
)

const (
	defaultMprisService = "org.mpris.MediaPlayer2.spotify"
	mprisPath           = "/org/mpris/MediaPlayer2"
	mprisPlayerIface    = "org.mpris.MediaPlayer2.Player"
	defaultLrclibGetURL = "https://lrclib.net/api/get"
	httpTimeoutSeconds  = 10
	pollInterval        = 100 * time.Millisecond
)

type trackInfo struct {
	Title        string
	Artist       string
	Album        string
	DurationSecs int64
	ArtworkURL   string
	TrackURL     string
	MprisTrackID string
}

type lrclibResponse struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

type timedLine struct {
	TimeSeconds float64
	Text        string
}

type tickMsg time.Time

type colorPalette struct {
	primary   string
	secondary string
	accent    string
	dim       string
}

type lyricsModel struct {
	bus          *dbus.Conn
	service      string
	lrclibURL    string
	syncOffset   float64
	track        *trackInfo
	lines        []timedLine
	currentIndex int
	err          error
	quitting     bool
	width        int
	height       int
	palette      *colorPalette
}

func (m lyricsModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m lyricsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k", "+", "=":
			m.syncOffset += 0.1
			if len(m.lines) > 0 {
				pos, _ := getCurrentPositionSeconds(m.bus, m.service)
				adjustedPos := float64(pos) + m.syncOffset
				m.currentIndex = findCurrentLineIndex(m.lines, adjustedPos)
			}
			return m, nil
		case "down", "j", "-":
			m.syncOffset -= 0.1
			if len(m.lines) > 0 {
				pos, _ := getCurrentPositionSeconds(m.bus, m.service)
				adjustedPos := float64(pos) + m.syncOffset
				m.currentIndex = findCurrentLineIndex(m.lines, adjustedPos)
			}
			return m, nil
		case "left", "h":
			m.syncOffset -= 0.5
			if len(m.lines) > 0 {
				pos, _ := getCurrentPositionSeconds(m.bus, m.service)
				adjustedPos := float64(pos) + m.syncOffset
				m.currentIndex = findCurrentLineIndex(m.lines, adjustedPos)
			}
			return m, nil
		case "right", "l":
			m.syncOffset += 0.5
			if len(m.lines) > 0 {
				pos, _ := getCurrentPositionSeconds(m.bus, m.service)
				adjustedPos := float64(pos) + m.syncOffset
				m.currentIndex = findCurrentLineIndex(m.lines, adjustedPos)
			}
			return m, nil
		case "0":
			m.syncOffset = 0
			if len(m.lines) > 0 {
				pos, _ := getCurrentPositionSeconds(m.bus, m.service)
				adjustedPos := float64(pos) + m.syncOffset
				m.currentIndex = findCurrentLineIndex(m.lines, adjustedPos)
			}
			return m, nil
		}

	case tickMsg:
		newTrack, err := getCurrentTrack(m.bus, m.service)
		if err != nil {
			return m, tickCmd()
		}

		if newTrack.MprisTrackID != m.track.MprisTrackID {
			m.track = newTrack
			
			if m.track.ArtworkURL != "" {
				m.palette = fetchAndExtractPalette(m.track.ArtworkURL)
			} else {
				m.palette = getDefaultPalette()
			}
			
			lyricsData, fetchErr := fetchLyrics(context.Background(), m.lrclibURL, m.track)
			if fetchErr != nil {
				m.err = fetchErr
				return m, tickCmd()
			}
			if lyricsData.SyncedLyrics == "" {
				m.err = errors.New("no synced lyrics available")
				return m, tickCmd()
			}
			m.lines = parseSyncedLyrics(lyricsData.SyncedLyrics)
			if len(m.lines) == 0 {
				m.err = errors.New("no synced lyrics available")
				return m, tickCmd()
			}
			m.err = nil
		}

		pos, err := getCurrentPositionSeconds(m.bus, m.service)
		if err != nil {
			return m, tickCmd()
		}

		adjustedPos := float64(pos) + m.syncOffset
		idx := findCurrentLineIndex(m.lines, adjustedPos)
		if idx != m.currentIndex {
			m.currentIndex = idx
		}

		return m, tickCmd()
	}

	return m, nil
}

func (m lyricsModel) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	palette := m.palette
	if palette == nil {
		palette = getDefaultPalette()
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(palette.secondary)).
		Align(lipgloss.Left).
		MarginLeft(2)

	currentLineStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(palette.primary)).
		Align(lipgloss.Center).
		Width(m.width)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Align(lipgloss.Center).
		Width(m.width)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(palette.dim)).
		Align(lipgloss.Right).
		MarginRight(2)

	syncStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C")).
		Align(lipgloss.Center).
		Width(m.width)

	if m.track == nil {
		return centerVertically(m.height, errorStyle.Render("waiting for track..."))
	}

	var lines []string

	header := fmt.Sprintf("%s - %s", m.track.Artist, m.track.Title)
	lines = append(lines, headerStyle.Render(header))

	if m.err != nil {
		for i := 0; i < (m.height-4)/2; i++ {
			lines = append(lines, "")
		}
		lines = append(lines, errorStyle.Render(m.err.Error()))
		for i := 0; i < (m.height-len(lines)-2); i++ {
			lines = append(lines, "")
		}
		lines = append(lines, helpStyle.Render("q to quit"))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	if m.currentIndex < 0 || m.currentIndex >= len(m.lines) {
		msgText := "waiting for first lyric..."
		if m.currentIndex >= len(m.lines) {
			msgText = "end of lyrics"
		}
		for i := 0; i < (m.height-4)/2; i++ {
			lines = append(lines, "")
		}
		dimStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Align(lipgloss.Center).
			Width(m.width)
		lines = append(lines, dimStyle.Render(msgText))
		for i := 0; i < (m.height-len(lines)-2); i++ {
			lines = append(lines, "")
		}
		lines = append(lines, helpStyle.Render("q to quit"))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	contextBefore := 4
	contextAfter := 4
	centerLine := (m.height - 2) / 2

	for i := 0; i < centerLine-contextBefore-1; i++ {
		lines = append(lines, "")
	}

	for i := contextBefore; i > 0; i-- {
		idx := m.currentIndex - i
		if idx >= 0 && idx < len(m.lines) {
			text := m.lines[idx].Text
			if text == "" {
				text = "..."
			}

			var color string
			switch i {
			case 1:
				color = palette.accent
			case 2:
				color = palette.secondary
			case 3:
				color = palette.dim
			default:
				color = "#44475A"
			}

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Align(lipgloss.Center).
				Width(m.width)
			lines = append(lines, style.Render(text))
		} else {
			lines = append(lines, "")
		}
	}

	currentText := m.lines[m.currentIndex].Text
	if currentText == "" {
		currentText = "..."
	}
	lines = append(lines, "")
	lines = append(lines, currentLineStyle.Render(currentText))
	lines = append(lines, "")

	for i := 1; i <= contextAfter; i++ {
		idx := m.currentIndex + i
		if idx >= 0 && idx < len(m.lines) {
			text := m.lines[idx].Text
			if text == "" {
				text = "..."
			}

			var color string
			switch i {
			case 1:
				color = palette.accent
			case 2:
				color = palette.secondary
			case 3:
				color = palette.dim
			default:
				color = "#44475A"
			}

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Align(lipgloss.Center).
				Width(m.width)
			lines = append(lines, style.Render(text))
		} else {
			lines = append(lines, "")
		}
	}

	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}

	if m.syncOffset != 0 {
		syncText := fmt.Sprintf("sync: %+.1fs", m.syncOffset)
		lines = append(lines, syncStyle.Render(syncText))
	} else {
		lines = append(lines, "")
	}

	helpText := "↑/↓ ±0.1s • ←/→ ±0.5s • 0 reset • q quit"
	lines = append(lines, helpStyle.Render(helpText))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func centerVertically(height int, content string) string {
	contentLines := strings.Split(content, "\n")
	contentHeight := len(contentLines)

	if contentHeight >= height {
		return content
	}

	topPadding := (height - contentHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	var result strings.Builder
	for i := 0; i < topPadding; i++ {
		result.WriteString("\n")
	}
	result.WriteString(content)

	return result.String()
}

func main() {
	ctx := context.Background()
	err := run(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	mprisService := getEnvOrDefault("MPRIS_SERVICE", defaultMprisService)
	lrclibURL := getEnvOrDefault("LRCLIB_GET_URL", defaultLrclibGetURL)
	syncOffsetStr := getEnvOrDefault("SYNC_OFFSET", "0")
	syncOffset, err := strconv.ParseFloat(syncOffsetStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid SYNC_OFFSET %q, using 0\n", syncOffsetStr)
		syncOffset = 0
	}

	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer bus.Close()

	track, err := getCurrentTrack(bus, mprisService)
	if err != nil {
		return err
	}

	positionSeconds, err := getCurrentPositionSeconds(bus, mprisService)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not read position:", err)
		positionSeconds = 0
	}

	lyricsData, err := fetchLyrics(ctx, lrclibURL, track)
	if err != nil {
		return err
	}

	var lines []timedLine
	var currentIndex int
	currentIndex = -1

	if lyricsData.SyncedLyrics != "" {
		lines = parseSyncedLyrics(lyricsData.SyncedLyrics)
		if len(lines) > 0 {
			adjustedPos := float64(positionSeconds) + syncOffset
			currentIndex = findCurrentLineIndex(lines, adjustedPos)
			if currentIndex < 0 && len(lines) > 0 {
				currentIndex = 0
			}
		}
	}

	if len(lines) > 0 {
		var palette *colorPalette
		if track.ArtworkURL != "" {
			palette = fetchAndExtractPalette(track.ArtworkURL)
		} else {
			palette = getDefaultPalette()
		}

		model := lyricsModel{
			bus:          bus,
			service:      mprisService,
			lrclibURL:    lrclibURL,
			syncOffset:   syncOffset,
			track:        track,
			lines:        lines,
			currentIndex: currentIndex,
			palette:      palette,
		}

		p := tea.NewProgram(
			model,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running bubble tea: %w", err)
		}
		return nil
	}

	printResult(track, lyricsData, positionSeconds, lines, currentIndex)

	return nil
}

func getEnvOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getCurrentTrack(bus *dbus.Conn, service string) (*trackInfo, error) {
	if bus == nil {
		return nil, errors.New("nil dbus connection")
	}
	if service == "" {
		return nil, errors.New("empty mpris service name")
	}

	obj := bus.Object(service, mprisPath)
	if obj == nil {
		return nil, errors.New("nil dbus object")
	}

	prop, err := obj.GetProperty(mprisPlayerIface + ".Metadata")
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata property: %w", err)
	}

	value := prop.Value()
	if value == nil {
		return nil, errors.New("metadata value is nil")
	}

	metadata, ok := value.(map[string]dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("unexpected metadata type %T", value)
	}

	track := &trackInfo{}
	track.Title = extractString(metadata, "xesam:title")
	track.Artist = extractArtist(metadata, "xesam:artist")
	track.Album = extractString(metadata, "xesam:album")
	track.ArtworkURL = extractString(metadata, "mpris:artUrl")
	track.TrackURL = extractString(metadata, "xesam:url")
	track.MprisTrackID = extractString(metadata, "mpris:trackid")
	track.DurationSecs = extractDurationSeconds(metadata, "mpris:length")

	if track.Title == "" || track.Artist == "" {
		return nil, fmt.Errorf("missing title or artist in metadata (title=%q, artist=%q)", track.Title, track.Artist)
	}

	return track, nil
}

func extractString(metadata map[string]dbus.Variant, key string) string {
	if metadata == nil {
		return ""
	}

	variant, exists := metadata[key]
	if !exists {
		return ""
	}

	raw := variant.Value()
	if raw == nil {
		return ""
	}

	text, ok := raw.(string)
	if ok {
		return text
	}

	return ""
}

func extractArtist(metadata map[string]dbus.Variant, key string) string {
	if metadata == nil {
		return ""
	}

	variant, exists := metadata[key]
	if !exists {
		return ""
	}

	raw := variant.Value()
	if raw == nil {
		return ""
	}

	switch typed := raw.(type) {
	case []string:
		if len(typed) > 0 {
			return typed[0]
		}
		return ""
	case string:
		return typed
	default:
		return ""
	}
}

func extractDurationSeconds(metadata map[string]dbus.Variant, key string) int64 {
	if metadata == nil {
		return 0
	}

	variant, exists := metadata[key]
	if !exists {
		return 0
	}

	raw := variant.Value()
	if raw == nil {
		return 0
	}

	switch typed := raw.(type) {
	case int64:
		if typed <= 0 {
			return 0
		}
		return typed / 1_000_000
	case uint64:
		if typed == 0 {
			return 0
		}
		return int64(typed / 1_000_000)
	default:
		return 0
	}
}

func getCurrentPositionSeconds(bus *dbus.Conn, service string) (int64, error) {
	if bus == nil {
		return 0, errors.New("nil dbus connection")
	}
	if service == "" {
		return 0, errors.New("empty mpris service name")
	}

	obj := bus.Object(service, mprisPath)
	if obj == nil {
		return 0, errors.New("nil dbus object")
	}

	prop, err := obj.GetProperty(mprisPlayerIface + ".Position")
	if err != nil {
		return 0, fmt.Errorf("failed to get position property: %w", err)
	}

	value := prop.Value()
	if value == nil {
		return 0, errors.New("position value is nil")
	}

	positionMicroseconds, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected position type %T", value)
	}
	if positionMicroseconds < 0 {
		return 0, nil
	}

	return positionMicroseconds / 1_000_000, nil
}

func fetchLyrics(parentCtx context.Context, baseURL string, track *trackInfo) (*lrclibResponse, error) {
	if track == nil {
		return nil, errors.New("nil track info")
	}
	if track.Title == "" || track.Artist == "" {
		return nil, errors.New("track title or artist is empty")
	}
	if baseURL == "" {
		return nil, errors.New("lrclib base url is empty")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid lrclib url %q: %w", baseURL, err)
	}

	query := parsedURL.Query()
	query.Set("artist_name", track.Artist)
	query.Set("track_name", track.Title)
	if track.Album != "" {
		query.Set("album_name", track.Album)
	}
	if track.DurationSecs > 0 {
		query.Set("duration", fmt.Sprintf("%d", track.DurationSecs))
	}
	parsedURL.RawQuery = query.Encode()

	timeout := time.Duration(httpTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build http request: %w", err)
	}

	req.Header.Set("User-Agent", "lyrecho/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("lrclib returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read lrclib response: %w", err)
	}

	var payload lrclibResponse
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode lrclib json: %w", err)
	}

	if payload.PlainLyrics == "" && payload.SyncedLyrics == "" && !payload.Instrumental {
		return nil, fmt.Errorf("no lyrics found for %s - %s", track.Artist, track.Title)
	}

	return &payload, nil
}

func parseSyncedLyrics(raw string) []timedLine {
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	result := make([]timedLine, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		timePart, text := splitLrcLine(trimmed)
		if timePart == "" || text == "" {
			continue
		}

		seconds, err := parseLrcTimeToSeconds(timePart)
		if err != nil {
			continue
		}

		result = append(result, timedLine{
			TimeSeconds: seconds,
			Text:        text,
		})
	}

	return result
}

func splitLrcLine(line string) (string, string) {
	if !strings.HasPrefix(line, "[") {
		return "", ""
	}

	endIndex := strings.Index(line, "]")
	if endIndex <= 1 {
		return "", ""
	}

	timePart := line[1:endIndex]
	textPart := strings.TrimSpace(line[endIndex+1:])
	if textPart == "" {
		return "", ""
	}

	return timePart, textPart
}

func parseLrcTimeToSeconds(raw string) (float64, error) {
	if raw == "" {
		return 0, errors.New("empty time value")
	}

	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid time format: %s", raw)
	}

	var hours float64
	var minutes float64
	var seconds float64
	var err error

	if len(parts) == 3 {
		hours, err = parseFloatSafe(parts[0])
		if err != nil {
			return 0, err
		}
		minutes, err = parseFloatSafe(parts[1])
		if err != nil {
			return 0, err
		}
		seconds, err = parseFloatSafe(parts[2])
		if err != nil {
			return 0, err
		}
	} else {
		hours = 0
		minutes, err = parseFloatSafe(parts[0])
		if err != nil {
			return 0, err
		}
		seconds, err = parseFloatSafe(parts[1])
		if err != nil {
			return 0, err
		}
	}

	total := hours*3600 + minutes*60 + seconds
	if total < 0 {
		return 0, errors.New("negative time not allowed")
	}

	return total, nil
}

func parseFloatSafe(s string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse float %q: %w", s, err)
	}
	return value, nil
}

func findCurrentLineIndex(lines []timedLine, positionSeconds float64) int {
	if len(lines) == 0 {
		return -1
	}

	index := -1

	for i, line := range lines {
		if line.TimeSeconds <= positionSeconds {
			index = i
			continue
		}
		break
	}

	return index
}

func fetchArtwork(artworkURL string) (image.Image, error) {
	if artworkURL == "" {
		return nil, errors.New("empty artwork url")
	}

	if strings.HasPrefix(artworkURL, "file://") {
		path := strings.TrimPrefix(artworkURL, "file://")
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open artwork file: %w", err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("failed to decode artwork image: %w", err)
		}
		return img, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artworkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artwork: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artwork fetch returned status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode artwork: %w", err)
	}

	return img, nil
}

func extractPalette(img image.Image) (*colorPalette, error) {
	if img == nil {
		return getDefaultPalette(), nil
	}

	colors, err := prominentcolor.KmeansWithAll(5, img, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, nil)
	if err != nil {
		return getDefaultPalette(), nil
	}

	if len(colors) < 3 {
		return getDefaultPalette(), nil
	}

	type colorWithMetrics struct {
		color      prominentcolor.ColorItem
		sat        float64
		brightness float64
		score      float64
	}

	colorMetrics := make([]colorWithMetrics, len(colors))
	for i, c := range colors {
		r := float64(c.Color.R) / 255.0
		g := float64(c.Color.G) / 255.0
		b := float64(c.Color.B) / 255.0

		max := math.Max(math.Max(r, g), b)
		min := math.Min(math.Min(r, g), b)
		
		var sat float64
		if max == 0 {
			sat = 0
		} else {
			sat = (max - min) / max
		}

		brightness := max

		score := sat * (1.0 - math.Abs(brightness-0.6))

		colorMetrics[i] = colorWithMetrics{
			color:      c,
			sat:        sat,
			brightness: brightness,
			score:      score,
		}
	}

	var primary, secondary, accent colorWithMetrics
	maxScore := -1.0
	
	for _, cm := range colorMetrics {
		if cm.score > maxScore && cm.brightness > 0.3 && cm.sat > 0.2 {
			maxScore = cm.score
			primary = cm
		}
	}

	for _, cm := range colorMetrics {
		if cm.color.Color.R != primary.color.Color.R || 
		   cm.color.Color.G != primary.color.Color.G || 
		   cm.color.Color.B != primary.color.Color.B {
			if cm.sat > 0.15 && cm.brightness > 0.3 {
				secondary = cm
				break
			}
		}
	}

	for _, cm := range colorMetrics {
		if (cm.color.Color.R != primary.color.Color.R || 
		    cm.color.Color.G != primary.color.Color.G || 
		    cm.color.Color.B != primary.color.Color.B) &&
		   (cm.color.Color.R != secondary.color.Color.R || 
		    cm.color.Color.G != secondary.color.Color.G || 
		    cm.color.Color.B != secondary.color.Color.B) {
			if cm.sat > 0.1 && cm.brightness > 0.25 {
				accent = cm
				break
			}
		}
	}

	primaryColor := boostColor(primary.color.Color.R, primary.color.Color.G, primary.color.Color.B, primary.brightness)
	secondaryColor := boostColor(secondary.color.Color.R, secondary.color.Color.G, secondary.color.Color.B, secondary.brightness)
	accentColor := boostColor(accent.color.Color.R, accent.color.Color.G, accent.color.Color.B, accent.brightness)

	palette := &colorPalette{
		primary:   primaryColor,
		secondary: secondaryColor,
		accent:    accentColor,
		dim:       "#6272A4",
	}

	return palette, nil
}

func boostColor(r, g, b uint32, brightness float64) string {
	if brightness < 0.4 {
		factor := 0.4 / brightness
		if factor > 2.5 {
			factor = 2.5
		}
		r = uint32(math.Min(255, float64(r)*factor))
		g = uint32(math.Min(255, float64(g)*factor))
		b = uint32(math.Min(255, float64(b)*factor))
	}
	
	if brightness > 0.85 {
		avg := (r + g + b) / 3
		factor := 0.7
		r = uint32(float64(avg) + (float64(r)-float64(avg))*factor)
		g = uint32(float64(avg) + (float64(g)-float64(avg))*factor)
		b = uint32(float64(avg) + (float64(b)-float64(avg))*factor)
	}

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func getDefaultPalette() *colorPalette {
	return &colorPalette{
		primary:   "#50FA7B",
		secondary: "#8BE9FD",
		accent:    "#BD93F9",
		dim:       "#6272A4",
	}
}

func fetchAndExtractPalette(artworkURL string) *colorPalette {
	img, err := fetchArtwork(artworkURL)
	if err != nil {
		return getDefaultPalette()
	}

	palette, err := extractPalette(img)
	if err != nil {
		return getDefaultPalette()
	}

	return palette
}

func formatTime(seconds int64) string {
	if seconds < 0 {
		return "0:00"
	}
	minutes := seconds / 60
	remaining := seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, remaining)
}

func formatTimeFromFloat(seconds float64) string {
	if seconds < 0 {
		return "0:00"
	}
	rounded := int64(math.Round(seconds))
	return formatTime(rounded)
}

func printResult(track *trackInfo, lyricsData *lrclibResponse, positionSecs int64, lines []timedLine, currentIndex int) {
	if track == nil {
		fmt.Println("no track info")
		return
	}
	if lyricsData == nil {
		fmt.Println("no lyrics data")
		return
	}

	fmt.Printf("%s - %s\n", track.Artist, track.Title)
	if track.Album != "" {
		fmt.Printf("album: %s\n", track.Album)
	}
	if track.TrackURL != "" {
		fmt.Printf("url: %s\n", track.TrackURL)
	}

	if positionSecs > 0 && track.DurationSecs > 0 {
		clamped := positionSecs
		if clamped > track.DurationSecs {
			clamped = track.DurationSecs
		}
		percent := 100 * float64(clamped) / float64(track.DurationSecs)
		fmt.Printf("position: %s / %s (%.0f%%)\n",
			formatTime(clamped),
			formatTime(track.DurationSecs),
			percent,
		)
	} else if positionSecs > 0 {
		fmt.Printf("position: %s\n", formatTime(positionSecs))
	}

	fmt.Println()

	if len(lines) > 0 && currentIndex >= 0 {
		fmt.Println("current position in synced lyrics:")
		fmt.Println()

		start := currentIndex - 3
		if start < 0 {
			start = 0
		}
		end := currentIndex + 3
		if end >= len(lines) {
			end = len(lines) - 1
		}

		for i := start; i <= end; i++ {
			prefix := "  "
			if i == currentIndex {
				prefix = "> "
			}
			fmt.Printf("%s[%s] %s\n",
				prefix,
				formatTimeFromFloat(lines[i].TimeSeconds),
				lines[i].Text,
			)
		}
		fmt.Println()
	} else {
		fmt.Println("no synced lyrics available or no position; showing full lyrics only.")
		fmt.Println()
	}

	if lyricsData.PlainLyrics != "" {
		fmt.Println(lyricsData.PlainLyrics)
	} else if lyricsData.SyncedLyrics != "" {
		fmt.Println(lyricsData.SyncedLyrics)
	} else if lyricsData.Instrumental {
		fmt.Println("[instrumental track]")
	} else {
		fmt.Println("[no lyrics available]")
	}
}
