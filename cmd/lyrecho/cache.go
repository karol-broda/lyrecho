package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"karolbroda.com/lyrecho/internal/cache"
)

var (
	// flags for cache list
	cacheSortBy string
	cacheConfirm bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "manage the lyrics cache",
	Long:  `manage cached lyrics data, including viewing statistics, listing entries, and clearing the cache.`,
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "show cache statistics",
	Long:  `display cache statistics including number of entries, total size, and cache location.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		diskCache := cache.GetGlobalCache()

		count, sizeBytes, err := diskCache.Stats()
		if err != nil {
			return fmt.Errorf("failed to get cache stats: %w", err)
		}

		// get cache directory path
		cacheDir := getCacheDir()

		fmt.Println("cache statistics:")
		fmt.Printf("  location: %s\n", cacheDir)
		fmt.Printf("  entries:  %d\n", count)
		fmt.Printf("  size:     %s\n", formatBytes(sizeBytes))

		return nil
	},
}

var cacheListCmd = &cobra.Command{
	Use:   "list",
	Short: "list all cached songs",
	Long:  `list all songs in the cache with their sync offsets and cache date.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		diskCache := cache.GetGlobalCache()

		entries, err := getAllCacheEntries(diskCache)
		if err != nil {
			return fmt.Errorf("failed to list cache: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("cache is empty")
			return nil
		}

		// sort entries
		sortCacheEntries(entries, cacheSortBy)

		// display as table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ARTIST\tTITLE\tSYNC OFFSET\tCACHED")

		for _, entry := range entries {
			syncStr := fmt.Sprintf("%.1fs", entry.SyncOffset)
			if entry.SyncOffset == 0 {
				syncStr = "-"
			}
			cacheDate := time.Unix(entry.CreatedAt, 0).Format("2006-01-02")
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", entry.ArtistName, entry.TrackName, syncStr, cacheDate)
		}

		w.Flush()

		fmt.Printf("\ntotal: %d songs\n", len(entries))

		return nil
	},
}

var cacheShowCmd = &cobra.Command{
	Use:   "show <artist> <title>",
	Short: "show cached entry for specific song",
	Long:  `display detailed information about a cached song including lyrics and sync offset.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		artist := args[0]
		title := args[1]

		diskCache := cache.GetGlobalCache()
		entry, err := diskCache.Get(artist, title)
		if err != nil {
			suggestions := findSimilarCachedSongs(diskCache, artist, title)
			if len(suggestions) > 0 {
				fmt.Fprintf(os.Stderr, "song not found in cache\n\n")
				fmt.Fprintf(os.Stderr, "did you mean one of these?\n")
				for _, s := range suggestions {
					fmt.Fprintf(os.Stderr, "  %s - %s\n", s.ArtistName, s.TrackName)
				}
				return fmt.Errorf("")
			}
			return fmt.Errorf("song not found in cache: %w", err)
		}

		fmt.Printf("artist:       %s\n", entry.ArtistName)
		fmt.Printf("title:        %s\n", entry.TrackName)
		fmt.Printf("album:        %s\n", entry.AlbumName)
		fmt.Printf("duration:     %.1fs\n", entry.Duration)
		fmt.Printf("sync offset:  %.2fs\n", entry.SyncOffset)
		fmt.Printf("instrumental: %v\n", entry.Instrumental)
		fmt.Printf("cached:       %s\n", time.Unix(entry.CreatedAt, 0).Format("2006-01-02 15:04:05"))
		fmt.Printf("expires:      %s\n", time.Unix(entry.ExpiresAt, 0).Format("2006-01-02 15:04:05"))

		if entry.SyncedLyrics != "" {
			lines := strings.Split(entry.SyncedLyrics, "\n")
			fmt.Printf("\nsynced lyrics: %d lines\n", len(lines))
		} else if entry.PlainLyrics != "" {
			lines := strings.Split(entry.PlainLyrics, "\n")
			fmt.Printf("\nplain lyrics: %d lines (no sync data)\n", len(lines))
		} else {
			fmt.Println("\nno lyrics available")
		}

		return nil
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "clear all cached entries",
	Long:  `remove all cached lyrics data. use --confirm to skip confirmation prompt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		diskCache := cache.GetGlobalCache()

		if !cacheConfirm {
			fmt.Print("are you sure you want to clear all cache? (y/n): ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				fmt.Println("cancelled")
				return nil
			}
		}

		err := diskCache.Clear()
		if err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}

		fmt.Println("cache cleared successfully")
		return nil
	},
}

var cachePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "remove expired cache entries",
	Long:  `remove all expired cache entries to free up disk space.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		diskCache := cache.GetGlobalCache()

		pruned, err := diskCache.Prune()
		if err != nil {
			return fmt.Errorf("failed to prune cache: %w", err)
		}

		fmt.Printf("removed %d expired entries\n", pruned)
		return nil
	},
}

var cacheDeleteCmd = &cobra.Command{
	Use:   "delete <artist> <title>",
	Short: "remove specific song from cache",
	Long:  `remove a specific song from the cache by artist and title.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		artist := args[0]
		title := args[1]

		diskCache := cache.GetGlobalCache()

		// verify it exists first
		_, err := diskCache.Get(artist, title)
		if err != nil {
			suggestions := findSimilarCachedSongs(diskCache, artist, title)
			if len(suggestions) > 0 {
				fmt.Fprintf(os.Stderr, "song not found in cache\n\n")
				fmt.Fprintf(os.Stderr, "did you mean one of these?\n")
				for _, s := range suggestions {
					fmt.Fprintf(os.Stderr, "  %s - %s\n", s.ArtistName, s.TrackName)
				}
				return fmt.Errorf("")
			}
			return fmt.Errorf("song not found in cache")
		}

		// delete from cache
		err = diskCache.Delete(artist, title)
		if err != nil {
			return fmt.Errorf("failed to delete from cache: %w", err)
		}

		fmt.Printf("deleted '%s - %s' from cache\n", artist, title)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.AddCommand(cacheStatsCmd)
	cacheCmd.AddCommand(cacheListCmd)
	cacheCmd.AddCommand(cacheShowCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cachePruneCmd)
	cacheCmd.AddCommand(cacheDeleteCmd)

	// flags for cache list
	cacheListCmd.Flags().StringVar(&cacheSortBy, "sort", "date", "sort by: date, artist, title")

	// flags for cache clear
	cacheClearCmd.Flags().BoolVar(&cacheConfirm, "confirm", false, "skip confirmation prompt")
}

// helper functions

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func getCacheDir() string {
	xdgCache := os.Getenv("XDG_CACHE_HOME")
	if xdgCache != "" {
		return xdgCache + "/lyric-shower/lyrics"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.cache/lyric-shower/lyrics"
	}
	return home + "/.cache/lyric-shower/lyrics"
}

func getAllCacheEntries(diskCache *cache.DiskCache) ([]*cache.LyricEntry, error) {
	entries, err := diskCache.ListAll()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func sortCacheEntries(entries []*cache.LyricEntry, sortBy string) {
	switch sortBy {
	case "artist":
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].ArtistName) < strings.ToLower(entries[j].ArtistName)
		})
	case "title":
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].TrackName) < strings.ToLower(entries[j].TrackName)
		})
	case "date":
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt > entries[j].CreatedAt
		})
	}
}

func findSimilarCachedSongs(diskCache *cache.DiskCache, artist string, title string) []*cache.LyricEntry {
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
