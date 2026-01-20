package ui

import (
	"image"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"karolbroda.com/lyrecho/internal/artwork"
	"karolbroda.com/lyrecho/internal/config"
	"karolbroda.com/lyrecho/internal/lyrics"
	"karolbroda.com/lyrecho/internal/player"
	"karolbroda.com/lyrecho/internal/terminal"
	"karolbroda.com/lyrecho/internal/track"
)

type LoadingState int

const (
	LoadingNone LoadingState = iota
	LoadingLyrics
	LoadingArtwork
	LoadingBoth
)

func (l LoadingState) IsLoadingLyrics() bool {
	return l == LoadingLyrics || l == LoadingBoth
}

func (l LoadingState) IsLoadingArtwork() bool {
	return l == LoadingArtwork || l == LoadingBoth
}

type TickMsg time.Time

type TrackChangedMsg struct {
	Track *track.Info
}

type SeekedMsg struct {
	PositionSeconds int64
}

type PlaybackStateMsg struct {
	Playing bool
}

type ArtworkFetchedMsg struct {
	Image   image.Image
	Palette *artwork.Palette
	Err     error
}

type LyricsFetchedMsg struct {
	Lines      []lyrics.TimedLine
	SyncOffset float64
	Err        error
}

type PlayerEventMsg struct {
	Event player.EventData
}

type TrackDisplay struct {
	Track        *track.Info
	Image        image.Image
	Palette      *artwork.Palette
	Lines        []lyrics.TimedLine
	CurrentIndex int
	PrevIndex    int
}

type Model struct {
	player     *player.Service
	lrclibURL  string
	syncOffset float64
	hideHeader bool
	termCaps   *terminal.Capabilities

	display        TrackDisplay
	positionSecs   int64
	loadingState   LoadingState
	err            error
	quitting       bool
	width          int
	height         int
	lastLineChange time.Time
	tickCount      int
	animState      AnimState
}

type ModelConfig struct {
	Player     *player.Service
	LrclibURL  string
	SyncOffset float64
	HideHeader bool
	TermCaps   *terminal.Capabilities
}

func NewModel(cfg ModelConfig) Model {
	m := Model{
		player:         cfg.Player,
		lrclibURL:      cfg.LrclibURL,
		syncOffset:     cfg.SyncOffset,
		hideHeader:     cfg.HideHeader,
		termCaps:       cfg.TermCaps,
		lastLineChange: time.Now(),
	}

	m.display.CurrentIndex = -1
	m.display.Palette = artwork.DefaultPalette()

	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tickCmd(),
		m.listenForPlayerEvents(),
	}

	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(config.PollInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) listenForPlayerEvents() tea.Cmd {
	if m.player == nil {
		return nil
	}

	return func() tea.Msg {
		event, ok := <-m.player.Events()
		if !ok {
			return nil
		}
		return PlayerEventMsg{Event: event}
	}
}

func (m *Model) setLoadingLyrics(loading bool) {
	if loading {
		if m.loadingState == LoadingArtwork {
			m.loadingState = LoadingBoth
		} else if m.loadingState == LoadingNone {
			m.loadingState = LoadingLyrics
		}
	} else {
		if m.loadingState == LoadingBoth {
			m.loadingState = LoadingArtwork
		} else if m.loadingState == LoadingLyrics {
			m.loadingState = LoadingNone
		}
	}
}

func (m *Model) setLoadingArtwork(loading bool) {
	if loading {
		if m.loadingState == LoadingLyrics {
			m.loadingState = LoadingBoth
		} else if m.loadingState == LoadingNone {
			m.loadingState = LoadingArtwork
		}
	} else {
		if m.loadingState == LoadingBoth {
			m.loadingState = LoadingLyrics
		} else if m.loadingState == LoadingArtwork {
			m.loadingState = LoadingNone
		}
	}
}

func (m *Model) resetForNewTrack() {
	m.display.Lines = nil
	m.display.CurrentIndex = -1
	m.display.PrevIndex = -1
	m.display.Image = nil
	m.display.Palette = artwork.DefaultPalette()
	m.lastLineChange = time.Now()
	m.err = nil
	m.animState.Reset()
}

func (m *Model) updateLyricIndex(positionSecs int64) bool {
	if len(m.display.Lines) == 0 {
		return false
	}

	adjustedPos := float64(positionSecs) + m.syncOffset
	idx := lyrics.FindCurrentLineIndex(m.display.Lines, adjustedPos)
	if idx < 0 && len(m.display.Lines) > 0 {
		idx = 0
	}

	if idx != m.display.CurrentIndex {
		m.display.PrevIndex = m.display.CurrentIndex
		m.display.CurrentIndex = idx
		m.lastLineChange = time.Now()
		m.animState.TargetScrollY = float64(idx)
		return true
	}

	return false
}

func (m Model) Width() int  { return m.width }
func (m Model) Height() int { return m.height }

func (m Model) Track() *track.Info        { return m.display.Track }
func (m Model) Position() int64           { return m.positionSecs }
func (m Model) Palette() *artwork.Palette { return m.display.Palette }
func (m Model) Image() image.Image        { return m.display.Image }
func (m Model) Lines() []lyrics.TimedLine { return m.display.Lines }
func (m Model) CurrentIndex() int         { return m.display.CurrentIndex }
func (m Model) SyncOffset() float64       { return m.syncOffset }
func (m Model) HideHeader() bool          { return m.hideHeader }
func (m Model) TickCount() int            { return m.tickCount }
func (m Model) LastLineChange() time.Time { return m.lastLineChange }
func (m Model) Err() error                { return m.err }
func (m Model) IsQuitting() bool          { return m.quitting }
func (m Model) IsLoadingLyrics() bool     { return m.loadingState.IsLoadingLyrics() }
func (m Model) IsLoadingArtwork() bool    { return m.loadingState.IsLoadingArtwork() }
func (m Model) AnimState() *AnimState     { return &m.animState }

func (m *Model) Stop() {
	if m.player != nil {
		m.player.Stop()
	}
}
