package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"karolbroda.com/lyrecho/internal/cache"
	"karolbroda.com/lyrecho/internal/config"
	"karolbroda.com/lyrecho/internal/lyrics"
)

var lyricsCmd = &cobra.Command{
	Use:   "lyrics",
	Short: "lyrics search and management",
	Long:  `search for lyrics, pre-fetch to cache, or preview lyrics in the terminal.`,
}

var lyricsSearchCmd = &cobra.Command{
	Use:   "search <artist> <title>",
	Short: "search for lyrics on lrclib",
	Long:  `search for lyrics on lrclib.net and display availability information.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		artist := args[0]
		title := args[1]

		cfg := config.Load()
		if lrclibURL != "" {
			cfg.LrclibURL = lrclibURL
		}

		fmt.Printf("searching for: %s - %s\n\n", artist, title)

		params := &lyrics.TrackParams{
			Title:  title,
			Artist: artist,
		}

		lyricsData, err := lyrics.Fetch(context.Background(), cfg.LrclibURL, params)
		if err != nil {
			return fmt.Errorf("lyrics not found: %w", err)
		}

		fmt.Printf("found lyrics:\n")
		fmt.Printf("  track:        %s\n", lyricsData.TrackName)
		fmt.Printf("  artist:       %s\n", lyricsData.ArtistName)
		if lyricsData.AlbumName != "" {
			fmt.Printf("  album:        %s\n", lyricsData.AlbumName)
		}
		if lyricsData.Duration > 0 {
			fmt.Printf("  duration:     %.0fs\n", lyricsData.Duration)
		}
		fmt.Printf("  instrumental: %v\n", lyricsData.Instrumental)

		if lyricsData.SyncedLyrics != "" {
			lines := strings.Split(lyricsData.SyncedLyrics, "\n")
			fmt.Printf("  synced lines: %d\n", len(lines))
		} else {
			fmt.Printf("  synced lines: none\n")
		}

		if lyricsData.PlainLyrics != "" {
			lines := strings.Split(lyricsData.PlainLyrics, "\n")
			fmt.Printf("  plain lines:  %d\n", len(lines))
		} else {
			fmt.Printf("  plain lines:  none\n")
		}

		fmt.Println("\nuse 'lyrecho lyrics fetch' to save to cache")

		return nil
	},
}

var lyricsFetchCmd = &cobra.Command{
	Use:   "fetch <artist> <title>",
	Short: "pre-fetch and cache lyrics",
	Long:  `fetch lyrics from lrclib.net and save them to the local cache for instant loading.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		artist := args[0]
		title := args[1]

		cfg := config.Load()
		if lrclibURL != "" {
			cfg.LrclibURL = lrclibURL
		}

		// check if already cached
		diskCache := cache.GetGlobalCache()
		cached, err := diskCache.Get(artist, title)
		if err == nil && cached != nil {
			fmt.Printf("'%s - %s' is already cached\n", artist, title)
			if cached.SyncOffset != 0 {
				fmt.Printf("sync offset: %.2fs\n", cached.SyncOffset)
			}
			return nil
		}

		fmt.Printf("fetching: %s - %s\n", artist, title)

		params := &lyrics.TrackParams{
			Title:  title,
			Artist: artist,
		}

		lyricsData, err := lyrics.Fetch(context.Background(), cfg.LrclibURL, params)
		if err != nil {
			return fmt.Errorf("failed to fetch lyrics: %w", err)
		}

		if lyricsData.SyncedLyrics == "" && lyricsData.PlainLyrics == "" {
			return fmt.Errorf("no lyrics available for this song")
		}

		fmt.Printf("cached successfully: %s - %s\n", lyricsData.ArtistName, lyricsData.TrackName)
		if lyricsData.SyncedLyrics != "" {
			fmt.Println("synced lyrics available")
		} else {
			fmt.Println("only plain lyrics available (no timing)")
		}

		return nil
	},
}

var lyricsPreviewCmd = &cobra.Command{
	Use:   "preview <artist> <title>",
	Short: "preview lyrics in terminal",
	Long:  `display lyrics in the terminal with timestamps (if available).`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		artist := args[0]
		title := args[1]

		cfg := config.Load()
		if lrclibURL != "" {
			cfg.LrclibURL = lrclibURL
		}

		// try cache first
		diskCache := cache.GetGlobalCache()
		cached, err := diskCache.Get(artist, title)

		var lyricsData *lyrics.LrclibResponse

		if err == nil && cached != nil {
			lyricsData = &lyrics.LrclibResponse{
				TrackName:    cached.TrackName,
				ArtistName:   cached.ArtistName,
				AlbumName:    cached.AlbumName,
				Duration:     cached.Duration,
				Instrumental: cached.Instrumental,
				PlainLyrics:  cached.PlainLyrics,
				SyncedLyrics: cached.SyncedLyrics,
				SyncOffset:   cached.SyncOffset,
			}
			fmt.Println("(from cache)")
		} else {
			// try fetching from lrclib
			params := &lyrics.TrackParams{
				Title:  title,
				Artist: artist,
			}

			lyricsData, err = lyrics.Fetch(context.Background(), cfg.LrclibURL, params)
			if err != nil {
				// check for similar songs in cache
				suggestions := findSimilarCachedSongsLyrics(diskCache, artist, title)
				if len(suggestions) > 0 {
					fmt.Fprintf(os.Stderr, "lyrics not found online\n\n")
					fmt.Fprintf(os.Stderr, "similar songs in cache:\n")
					for _, s := range suggestions {
						fmt.Fprintf(os.Stderr, "  %s - %s\n", s.ArtistName, s.TrackName)
					}
					return fmt.Errorf("")
				}
				return fmt.Errorf("lyrics not found: %w", err)
			}
		}

		fmt.Printf("\n%s - %s\n", lyricsData.ArtistName, lyricsData.TrackName)
		if lyricsData.AlbumName != "" {
			fmt.Printf("%s\n", lyricsData.AlbumName)
		}
		fmt.Println(strings.Repeat("─", 60))

		if lyricsData.Instrumental {
			fmt.Println("\n[instrumental]")
			return nil
		}

		if lyricsData.SyncedLyrics != "" {
			// display synced lyrics with timestamps
			lines := lyrics.ParseSynced(lyricsData.SyncedLyrics)
			if len(lines) == 0 {
				fmt.Println("\nno valid synced lyrics found")
				return nil
			}

			fmt.Printf("\nsynced lyrics (%d lines):\n\n", len(lines))
			for _, line := range lines {
				timestamp := formatTimestamp(line.TimeSeconds)
				fmt.Printf("[%s] %s\n", timestamp, line.Text)
			}

			if lyricsData.SyncOffset != 0 {
				fmt.Printf("\nsync offset: %.2fs\n", lyricsData.SyncOffset)
			}
		} else if lyricsData.PlainLyrics != "" {
			// display plain lyrics
			fmt.Println("\nplain lyrics (no timestamps):\n")
			fmt.Println(lyricsData.PlainLyrics)
		} else {
			fmt.Println("\nno lyrics available")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lyricsCmd)

	lyricsCmd.AddCommand(lyricsSearchCmd)
	lyricsCmd.AddCommand(lyricsFetchCmd)
	lyricsCmd.AddCommand(lyricsPreviewCmd)
}

// helper functions

func formatTimestamp(seconds float64) string {
	minutes := int(seconds) / 60
	secs := seconds - float64(minutes*60)
	return fmt.Sprintf("%d:%05.2f", minutes, secs)
}

func findSimilarCachedSongsLyrics(diskCache *cache.DiskCache, artist string, title string) []*cache.LyricEntry {
	allEntries, err := diskCache.ListAll()
	if err != nil || len(allEntries) == 0 {
		return nil
	}

	var matches []*cache.LyricEntry
	artistLower := strings.ToLower(artist)
	titleLower := strings.ToLower(title)

	// first pass: exact artist match with fuzzy title
	for _, entry := range allEntries {
		if strings.ToLower(entry.ArtistName) == artistLower {
			entryTitleLower := strings.ToLower(entry.TrackName)
			// check if titles are similar (contains or contained)
			if strings.Contains(entryTitleLower, titleLower) || strings.Contains(titleLower, entryTitleLower) {
				matches = append(matches, entry)
			}
		}
	}

	// if we found matches, return them (up to 5)
	if len(matches) > 0 {
		if len(matches) > 5 {
			matches = matches[:5]
		}
		return matches
	}

	// second pass: fuzzy artist match with fuzzy title
	for _, entry := range allEntries {
		entryArtistLower := strings.ToLower(entry.ArtistName)
		entryTitleLower := strings.ToLower(entry.TrackName)

		artistMatch := strings.Contains(entryArtistLower, artistLower) || strings.Contains(artistLower, entryArtistLower)
		titleMatch := strings.Contains(entryTitleLower, titleLower) || strings.Contains(titleLower, entryTitleLower)

		if artistMatch && titleMatch {
			matches = append(matches, entry)
		}
	}

	// return up to 5 suggestions
	if len(matches) > 5 {
		matches = matches[:5]
	}

	return matches
}
