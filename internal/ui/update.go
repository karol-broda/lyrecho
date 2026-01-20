package ui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"karolbroda.com/lyrecho/internal/artwork"
	"karolbroda.com/lyrecho/internal/cache"
	"karolbroda.com/lyrecho/internal/lyrics"
	"karolbroda.com/lyrecho/internal/player"
	"karolbroda.com/lyrecho/internal/track"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case PlayerEventMsg:
		return m.handlePlayerEvent(msg.Event)

	case ArtworkFetchedMsg:
		return m.handleArtworkFetched(msg)

	case LyricsFetchedMsg:
		return m.handleLyricsFetched(msg)

	case TickMsg:
		return m.handleTick()
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.quitting = true
		m.Stop()
		return m, tea.Quit

	case "up", "k", "+", "=":
		m.syncOffset += 0.1
		m.updateLyricIndexFromPosition()
		m.saveSyncOffset()
		return m, nil

	case "down", "j", "-":
		m.syncOffset -= 0.1
		m.updateLyricIndexFromPosition()
		m.saveSyncOffset()
		return m, nil

	case "left", "h":
		m.syncOffset -= 0.5
		m.updateLyricIndexFromPosition()
		m.saveSyncOffset()
		return m, nil

	case "right", "l":
		m.syncOffset += 0.5
		m.updateLyricIndexFromPosition()
		m.saveSyncOffset()
		return m, nil

	case "0":
		m.syncOffset = 0
		m.updateLyricIndexFromPosition()
		m.saveSyncOffset()
		return m, nil

	case "tab", "i":
		m.hideHeader = !m.hideHeader
		return m, nil
	}

	return m, nil
}

func (m *Model) saveSyncOffset() {
	if m.display.Track == nil {
		return
	}

	// update the cache entry with the new sync offset
	diskCache := cache.GetGlobalCache()

	// get existing cache entry
	cached, err := diskCache.Get(m.display.Track.Artist, m.display.Track.Title)
	if err != nil {
		// no cached entry yet, nothing to update
		return
	}

	// update sync offset
	cached.SyncOffset = m.syncOffset

	// save back to cache
	_ = diskCache.Set(m.display.Track.Artist, m.display.Track.Title, cached)
}

func (m *Model) updateLyricIndexFromPosition() {
	if m.player == nil {
		return
	}
	pos, err := m.player.GetCurrentPosition()
	if err != nil {
		return
	}
	m.updateLyricIndex(pos)
}

func (m Model) handlePlayerEvent(event player.EventData) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	cmds = append(cmds, m.listenForPlayerEvents())

	switch event.Type {
	case player.EventTrackChanged:
		return m.handleTrackChange(event.Track, cmds)

	case player.EventSeeked:
		m.positionSecs = event.Position
		m.updateLyricIndex(event.Position)
		m.lastLineChange = time.Now()
		m.animState.Reset()
		return m, tea.Batch(cmds...)

	case player.EventPlaybackStateChanged:
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleTrackChange(newTrack *track.Info, existingCmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.display.Track = newTrack
	m.resetForNewTrack()

	if newTrack == nil || !newTrack.IsValid() {
		m.err = errors.New("no track playing")
		return m, tea.Batch(existingCmds...)
	}

	if newTrack.ArtworkURL != "" {
		m.setLoadingArtwork(true)
		existingCmds = append(existingCmds, fetchArtworkCmd(newTrack.ArtworkURL))
	}

	m.setLoadingLyrics(true)
	existingCmds = append(existingCmds, fetchLyricsCmd(m.lrclibURL, newTrack))

	return m, tea.Batch(existingCmds...)
}

func (m Model) handleArtworkFetched(msg ArtworkFetchedMsg) (tea.Model, tea.Cmd) {
	m.setLoadingArtwork(false)

	if msg.Err == nil && msg.Image != nil {
		m.display.Image = msg.Image
		if msg.Palette != nil {
			m.display.Palette = msg.Palette
		}
	} else if msg.Err != nil {
		// artwork failed to load, but we should still update palette
		// use default palette explicitly to ensure colors are set
		if m.display.Palette == nil {
			m.display.Palette = artwork.DefaultPalette()
		}
	}

	// force a re-render by sending a no-op message
	// this ensures lyrics that have already loaded will re-render with the new palette
	return m, func() tea.Msg {
		return struct{}{}
	}
}

func (m Model) handleLyricsFetched(msg LyricsFetchedMsg) (tea.Model, tea.Cmd) {
	m.setLoadingLyrics(false)

	if msg.Err != nil {
		m.err = msg.Err
		m.display.Lines = nil
		m.display.CurrentIndex = -1
		return m, nil
	}

	if len(msg.Lines) == 0 {
		m.err = errors.New("no synced lyrics available")
		m.display.Lines = nil
		m.display.CurrentIndex = -1
		return m, nil
	}

	m.display.Lines = msg.Lines
	m.err = nil
	m.display.CurrentIndex = 0

	// restore cached sync offset for this song
	if msg.SyncOffset != 0 {
		m.syncOffset = msg.SyncOffset
	}

	return m, nil
}

func (m Model) handleTick() (tea.Model, tea.Cmd) {
	m.tickCount++

	if m.player == nil {
		m.animState.Update(m.tickCount, false, 8)
		return m, tickCmd()
	}

	err := m.player.Poll()
	if err != nil {
		m.animState.Update(m.tickCount, false, 8)
		return m, tickCmd()
	}

	pos, err := m.player.GetCurrentPosition()
	if err != nil {
		m.animState.Update(m.tickCount, false, 8)
		return m, tickCmd()
	}

	m.positionSecs = pos

	lineChanged := m.updateLyricIndex(pos)
	m.animState.Update(m.tickCount, lineChanged, 8)

	return m, tickCmd()
}

func fetchArtworkCmd(artworkURL string) tea.Cmd {
	return func() tea.Msg {
		img, err := artwork.Fetch(artworkURL)
		if err != nil {
			return ArtworkFetchedMsg{Err: err}
		}
		palette := artwork.ExtractPalette(img)
		return ArtworkFetchedMsg{
			Image:   img,
			Palette: palette,
		}
	}
}

func fetchLyricsCmd(lrclibURL string, trk *track.Info) tea.Cmd {
	return func() tea.Msg {
		if trk == nil {
			return LyricsFetchedMsg{Err: errors.New("nil track")}
		}

		params := &lyrics.TrackParams{
			Title:        trk.Title,
			Artist:       trk.Artist,
			Album:        trk.Album,
			DurationSecs: trk.DurationSecs,
		}

		lyricsData, err := lyrics.Fetch(context.Background(), lrclibURL, params)
		if err != nil {
			return LyricsFetchedMsg{Err: err}
		}

		if lyricsData.SyncedLyrics == "" {
			return LyricsFetchedMsg{Err: errors.New("no synced lyrics available")}
		}

		lines := lyrics.ParseSynced(lyricsData.SyncedLyrics)
		return LyricsFetchedMsg{Lines: lines, SyncOffset: lyricsData.SyncOffset}
	}
}
