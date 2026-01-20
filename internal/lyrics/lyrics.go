package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"karolbroda.com/lyrecho/internal/cache"
	"karolbroda.com/lyrecho/internal/config"
)

var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

type LrclibResponse struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
	SyncOffset   float64 `json:"-"`
}

type TimedLine struct {
	TimeSeconds float64
	Text        string
}

type TrackParams struct {
	Title        string
	Artist       string
	Album        string
	DurationSecs int64
}

func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     60 * time.Second,
			TLSHandshakeTimeout: 2 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   time.Duration(config.HTTPTimeoutSeconds) * time.Second,
		}
	})
	return httpClient
}

const maxRetries = 0

// normalizeString cleans and normalizes track/artist names for better matching
func normalizeString(s string) string {
	s = strings.TrimSpace(s)

	// remove multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}

	s = strings.TrimSpace(s)
	return s
}

// stripVersionInfo removes text in parentheses and brackets (remixes, versions, etc)
func stripVersionInfo(s string) string {
	s = strings.TrimSpace(s)

	// remove content within parentheses
	for strings.Contains(s, "(") && strings.Contains(s, ")") {
		start := strings.Index(s, "(")
		end := strings.Index(s, ")")
		if end > start {
			s = s[:start] + " " + s[end+1:]
		} else {
			break
		}
	}

	// remove content within brackets
	for strings.Contains(s, "[") && strings.Contains(s, "]") {
		start := strings.Index(s, "[")
		end := strings.Index(s, "]")
		if end > start {
			s = s[:start] + " " + s[end+1:]
		} else {
			break
		}
	}

	// remove multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}

	s = strings.TrimSpace(s)
	return s
}

// toTitleCase converts a string to title case (first letter of each word capitalized)
func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

func Fetch(parentCtx context.Context, baseURL string, track *TrackParams) (*LrclibResponse, error) {
	if track == nil {
		return nil, errors.New("nil track info")
	}
	if track.Title == "" || track.Artist == "" {
		return nil, errors.New("track title or artist is empty")
	}
	if baseURL == "" {
		return nil, errors.New("lrclib base url is empty")
	}

	diskCache := cache.GetGlobalCache()

	// normalize input for better matching
	normalizedArtist := normalizeString(track.Artist)
	normalizedTitle := normalizeString(track.Title)
	strippedArtist := stripVersionInfo(track.Artist)
	strippedTitle := stripVersionInfo(track.Title)

	if normalizedTitle == "" || normalizedArtist == "" {
		return nil, errors.New("track title or artist is empty after normalization")
	}

	// check persistent cache first (use original values for cache key)
	cached, err := diskCache.Get(track.Artist, track.Title)
	if err == nil && cached != nil {
		return &LrclibResponse{
			TrackName:    cached.TrackName,
			ArtistName:   cached.ArtistName,
			AlbumName:    cached.AlbumName,
			Duration:     cached.Duration,
			Instrumental: cached.Instrumental,
			PlainLyrics:  cached.PlainLyrics,
			SyncedLyrics: cached.SyncedLyrics,
			SyncOffset:   cached.SyncOffset,
		}, nil
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid lrclib url %q: %w", baseURL, err)
	}

	// build unique search strategies
	searchStrategies := []struct {
		artist   string
		title    string
		album    string
		duration int64
	}{
		// strategy 1: normalized names with album and duration
		{normalizedArtist, normalizedTitle, track.Album, track.DurationSecs},
		// strategy 2: normalized names without album
		{normalizedArtist, normalizedTitle, "", track.DurationSecs},
		// strategy 3: normalized names without album or duration
		{normalizedArtist, normalizedTitle, "", 0},
		// strategy 4: stripped version info (no parens/brackets) without album
		{strippedArtist, strippedTitle, "", 0},
		// strategy 6: uppercase (some artists like SURF CURSE)
		{strings.ToUpper(normalizedArtist), strings.ToUpper(normalizedTitle), "", 0},
		// strategy 7: lowercase
		{strings.ToLower(normalizedArtist), strings.ToLower(normalizedTitle), "", 0},
		// strategy 8: title case
		{toTitleCase(normalizedArtist), toTitleCase(normalizedTitle), "", 0},
		// strategy 5: original names without album or duration (fallback)
		{track.Artist, track.Title, "", 0},
	}

	// deduplicate strategies
	seen := make(map[string]bool)
	var uniqueStrategies []struct {
		artist   string
		title    string
		album    string
		duration int64
	}

	for _, strategy := range searchStrategies {
		if strategy.artist == "" || strategy.title == "" {
			continue
		}

		// create unique key for this search
		key := fmt.Sprintf("%s|%s|%s|%d", strategy.artist, strategy.title, strategy.album, strategy.duration)
		if !seen[key] {
			seen[key] = true
			uniqueStrategies = append(uniqueStrategies, strategy)
		}
	}

	var lastErr error
	for strategyIdx, strategy := range uniqueStrategies {

		query := parsedURL.Query()
		query.Set("artist_name", strategy.artist)
		query.Set("track_name", strategy.title)
		if strategy.album != "" {
			query.Set("album_name", strategy.album)
		}
		if strategy.duration > 0 {
			query.Set("duration", fmt.Sprintf("%d", strategy.duration))
		}
		parsedURL.RawQuery = query.Encode()

		// add small delay between strategies to avoid hammering the server
		if strategyIdx > 0 {
			select {
			case <-parentCtx.Done():
				return nil, parentCtx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}

		payload, err := doFetchRequest(parentCtx, parsedURL.String())
		if err == nil {
			if payload.PlainLyrics == "" && payload.SyncedLyrics == "" && !payload.Instrumental {
				// no lyrics in response, try next strategy
				lastErr = fmt.Errorf("no lyrics in response")
				continue
			}

			// found lyrics! persist to disk cache using original keys
			_ = diskCache.Set(track.Artist, track.Title, &cache.LyricEntry{
				TrackName:    payload.TrackName,
				ArtistName:   payload.ArtistName,
				AlbumName:    payload.AlbumName,
				Duration:     payload.Duration,
				Instrumental: payload.Instrumental,
				PlainLyrics:  payload.PlainLyrics,
				SyncedLyrics: payload.SyncedLyrics,
				SyncOffset:   payload.SyncOffset,
			})

			return payload, nil
		}

		lastErr = err

		// if this is a 404 or similar, try next strategy quickly
		// only give up immediately on actual network timeouts
		if isTimeoutError(err) {
			return nil, errors.New("lyrics server took too long to respond")
		}
	}

	// all strategies failed
	if lastErr != nil {
		return nil, fmt.Errorf("no lyrics found for %s - %s: %w", track.Artist, track.Title, lastErr)
	}
	return nil, fmt.Errorf("no lyrics found for %s - %s (tried multiple search variations)", track.Artist, track.Title)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded") ||
		strings.Contains(err.Error(), "i/o timeout")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "status 404") ||
		strings.Contains(err.Error(), "not found")
}

func doFetchRequest(parentCtx context.Context, requestURL string) (*LrclibResponse, error) {
	timeout := time.Duration(config.HTTPTimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build http request: %w", err)
	}

	req.Header.Set("User-Agent", "lyric-shower/1.0")

	client := getHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("status 404: lyrics not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("lrclib returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read lrclib response: %w", err)
	}

	var payload LrclibResponse
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode lrclib json: %w", err)
	}

	return &payload, nil
}

func ParseSynced(raw string) []TimedLine {
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	result := make([]TimedLine, 0, len(lines))

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

		result = append(result, TimedLine{
			TimeSeconds: seconds,
			Text:        text,
		})
	}

	return result
}

func FindCurrentLineIndex(lines []TimedLine, positionSeconds float64) int {
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
