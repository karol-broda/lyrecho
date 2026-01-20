package main

import (
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/spf13/cobra"

	"karolbroda.com/lyrecho/internal/config"
	"karolbroda.com/lyrecho/internal/player"
)

var (
	// flags for player test
	testService string
)

var playerCmd = &cobra.Command{
	Use:   "player",
	Short: "mpris player utilities",
	Long:  `discover and test mpris-compatible music players on your system.`,
}

var playerListCmd = &cobra.Command{
	Use:   "list",
	Short: "list available mpris players",
	Long:  `list all mpris-compatible music players currently running on the system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bus, err := dbus.ConnectSessionBus()
		if err != nil {
			return fmt.Errorf("failed to connect to session bus: %w", err)
		}
		defer bus.Close()

		// list all names on the session bus
		var names []string
		err = bus.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
		if err != nil {
			return fmt.Errorf("failed to list dbus names: %w", err)
		}

		// filter for mpris services
		var mprisServices []string
		for _, name := range names {
			if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
				mprisServices = append(mprisServices, name)
			}
		}

		if len(mprisServices) == 0 {
			fmt.Println("no mpris players found")
			fmt.Println("\ncheck if your music player is running and supports mpris")
			return nil
		}

		fmt.Printf("found %d mpris player(s):\n\n", len(mprisServices))
		for _, service := range mprisServices {
			// try to get player identity
			identity := getPlayerIdentity(bus, service)
			if identity != "" {
				fmt.Printf("  %s (%s)\n", service, identity)
			} else {
				fmt.Printf("  %s\n", service)
			}
		}

		fmt.Println("\nuse --mpris-service flag to specify which player to use")

		return nil
	},
}

var playerTestCmd = &cobra.Command{
	Use:   "test",
	Short: "test connection to mpris player",
	Long:  `test the connection to an mpris player and display basic information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()

		// use flag if provided, otherwise use config
		serviceName := cfg.MprisService
		if testService != "" {
			serviceName = testService
		}

		bus, err := dbus.ConnectSessionBus()
		if err != nil {
			return fmt.Errorf("failed to connect to session bus: %w", err)
		}
		defer bus.Close()

		fmt.Printf("testing connection to: %s\n\n", serviceName)

		// try to create player service
		playerService, err := player.NewService(bus, serviceName)
		if err != nil {
			return fmt.Errorf("failed to connect to player: %w", err)
		}

		// get player identity
		identity := getPlayerIdentity(bus, serviceName)
		if identity != "" {
			fmt.Printf("player identity: %s\n", identity)
		}

		// try to get current track
		state := playerService.GetState()
		if state.Track != nil && state.Track.IsValid() {
			fmt.Printf("status: connected ✓\n\n")
			fmt.Println("current track:")
			fmt.Printf("  title:  %s\n", state.Track.Title)
			fmt.Printf("  artist: %s\n", state.Track.Artist)
			if state.Track.Album != "" {
				fmt.Printf("  album:  %s\n", state.Track.Album)
			}
			if state.Playing {
				fmt.Printf("  state:  playing\n")
			} else {
				fmt.Printf("  state:  paused\n")
			}
		} else {
			fmt.Printf("status: connected ✓\n\n")
			fmt.Println("no track currently playing")
		}

		return nil
	},
}

var playerCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "show currently playing track",
	Long:  `display information about the currently playing track.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()

		if mprisService != "" {
			cfg.MprisService = mprisService
		}

		bus, err := dbus.ConnectSessionBus()
		if err != nil {
			return fmt.Errorf("failed to connect to session bus: %w", err)
		}
		defer bus.Close()

		playerService, err := player.NewService(bus, cfg.MprisService)
		if err != nil {
			return fmt.Errorf("failed to connect to player: %w", err)
		}

		state := playerService.GetState()
		if state.Track == nil || !state.Track.IsValid() {
			fmt.Println("no track currently playing")
			return nil
		}

		fmt.Printf("title:    %s\n", state.Track.Title)
		fmt.Printf("artist:   %s\n", state.Track.Artist)
		if state.Track.Album != "" {
			fmt.Printf("album:    %s\n", state.Track.Album)
		}
		if state.Track.DurationSecs > 0 {
			fmt.Printf("duration: %s\n", formatDuration(state.Track.DurationSecs))
		}
		if state.Track.ArtworkURL != "" {
			fmt.Printf("artwork:  %s\n", state.Track.ArtworkURL)
		}
		if state.Playing {
			fmt.Printf("state:    playing\n")
			if state.PositionSecs > 0 {
				fmt.Printf("position: %s\n", formatDuration(state.PositionSecs))
			}
		} else {
			fmt.Printf("state:    paused\n")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(playerCmd)

	playerCmd.AddCommand(playerListCmd)
	playerCmd.AddCommand(playerTestCmd)
	playerCmd.AddCommand(playerCurrentCmd)

	// flags for player test
	playerTestCmd.Flags().StringVar(&testService, "service", "", "mpris service to test")
}

// helper functions

func getPlayerIdentity(bus *dbus.Conn, serviceName string) string {
	obj := bus.Object(serviceName, "/org/mpris/MediaPlayer2")
	variant, err := obj.GetProperty("org.mpris.MediaPlayer2.Identity")
	if err != nil {
		return ""
	}

	identity, ok := variant.Value().(string)
	if !ok {
		return ""
	}

	return identity
}

func formatDuration(seconds int64) string {
	if seconds < 0 {
		return "0:00"
	}
	minutes := seconds / 60
	remaining := seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, remaining)
}
