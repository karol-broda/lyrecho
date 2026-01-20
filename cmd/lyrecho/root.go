package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// global flags
	mprisService string
	syncOffset   float64
	hideHeader   bool
	lrclibURL    string
	noCache      bool
)

var rootCmd = &cobra.Command{
	Use:   "lyrecho",
	Short: "terminal-based synchronized lyrics viewer",
	Long: `lyrecho is a terminal-based synchronized lyrics viewer for linux music players.
it displays real-time lyrics with dynamic color themes extracted from album artwork.

when run without a subcommand, it starts the interactive TUI viewer.`,
	Version: "1.0.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		// default behavior: run the TUI viewer
		return runViewer(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// global flags for the viewer
	rootCmd.PersistentFlags().StringVarP(&mprisService, "mpris-service", "m", "", "mpris service name (e.g., org.mpris.MediaPlayer2.spotify)")
	rootCmd.PersistentFlags().Float64VarP(&syncOffset, "sync-offset", "s", 0, "initial sync offset in seconds")
	rootCmd.PersistentFlags().BoolVarP(&hideHeader, "hide-header", "H", false, "hide header section")
	rootCmd.PersistentFlags().StringVar(&lrclibURL, "lrclib-url", "", "custom lrclib api url")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "disable cache reads (always fetch fresh)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
