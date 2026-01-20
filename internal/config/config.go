package config

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultMprisService = "org.mpris.MediaPlayer2.spotify"
	DefaultLrclibGetURL = "https://lrclib.net/api/get"
	HTTPTimeoutSeconds  = 10
	PollInterval        = 100 * time.Millisecond
)

type Config struct {
	MprisService string
	LrclibURL    string
	SyncOffset   float64
	HideHeader   bool
}

func Load() *Config {
	syncOffsetStr := getEnvOrDefault("SYNC_OFFSET", "0")
	syncOffset, err := strconv.ParseFloat(syncOffsetStr, 64)
	if err != nil {
		syncOffset = 0
	}

	hideHeaderStr := getEnvOrDefault("HIDE_HEADER", "false")
	hideHeader := hideHeaderStr == "1" || hideHeaderStr == "true" || hideHeaderStr == "yes"

	return &Config{
		MprisService: getEnvOrDefault("MPRIS_SERVICE", DefaultMprisService),
		LrclibURL:    getEnvOrDefault("LRCLIB_GET_URL", DefaultLrclibGetURL),
		SyncOffset:   syncOffset,
		HideHeader:   hideHeader,
	}
}

func getEnvOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
