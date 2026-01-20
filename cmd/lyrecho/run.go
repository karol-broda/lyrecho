package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
	"github.com/spf13/cobra"

	"karolbroda.com/lyrecho/internal/config"
	"karolbroda.com/lyrecho/internal/player"
	"karolbroda.com/lyrecho/internal/terminal"
	"karolbroda.com/lyrecho/internal/ui"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "start the interactive lyrics viewer",
	Long:  `starts the terminal-based lyrics viewer with real-time synchronized lyrics display.`,
	RunE:  runViewer,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runViewer(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigChan
		cancel()
		terminal.Reset()
		os.Exit(0)
	}()

	defer terminal.Reset()

	// load config from environment, then override with flags
	cfg := config.Load()

	if mprisService != "" {
		cfg.MprisService = mprisService
	}
	if lrclibURL != "" {
		cfg.LrclibURL = lrclibURL
	}
	if cmd.Flags().Changed("sync-offset") {
		cfg.SyncOffset = syncOffset
	}
	if cmd.Flags().Changed("hide-header") {
		cfg.HideHeader = hideHeader
	}

	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer bus.Close()

	playerService, err := player.NewService(bus, cfg.MprisService)
	if err != nil {
		return fmt.Errorf("failed to create player service: %w", err)
	}

	err = playerService.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not set up dbus signals: %v\n", err)
	}

	termCaps := terminal.DetectCapabilities()

	model := ui.NewModel(ui.ModelConfig{
		Player:     playerService,
		LrclibURL:  cfg.LrclibURL,
		SyncOffset: cfg.SyncOffset,
		HideHeader: cfg.HideHeader,
		TermCaps:   termCaps,
	})

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	go func() {
		<-ctx.Done()
		playerService.Stop()
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running bubble tea: %w", err)
	}

	return nil
}
