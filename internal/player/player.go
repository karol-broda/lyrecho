package player

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"karolbroda.com/lyrecho/internal/track"
)

const (
	mprisPath        = "/org/mpris/MediaPlayer2"
	mprisPlayerIface = "org.mpris.MediaPlayer2.Player"
)

type Event int

const (
	EventTrackChanged Event = iota
	EventPositionChanged
	EventSeeked
	EventPlaybackStateChanged
)

type EventData struct {
	Type     Event
	Track    *track.Info
	Position int64
	Playing  bool
}

type State struct {
	Track               *track.Info
	PositionSecs        int64
	Playing             bool
	lastPositionUpdate  time.Time
	lastPositionSecs    int64
}

func (s *State) DetectSeek(newPosition int64) bool {
	if s.lastPositionUpdate.IsZero() {
		return false
	}

	elapsed := time.Since(s.lastPositionUpdate)
	expectedPos := s.lastPositionSecs + int64(elapsed.Seconds())

	diff := newPosition - expectedPos
	if diff < 0 {
		diff = -diff
	}

	return diff > 3
}

func (s *State) UpdatePosition(pos int64) {
	s.PositionSecs = pos
	s.lastPositionSecs = pos
	s.lastPositionUpdate = time.Now()
}

type Service struct {
	bus        *dbus.Conn
	service    string
	signalChan chan *dbus.Signal
	stopChan   chan struct{}
	stopOnce   sync.Once
	eventChan  chan EventData
	state      *State
	mu         sync.RWMutex
}

func NewService(bus *dbus.Conn, mprisService string) (*Service, error) {
	if bus == nil {
		return nil, errors.New("nil dbus connection")
	}
	if mprisService == "" {
		return nil, errors.New("empty mpris service name")
	}

	s := &Service{
		bus:       bus,
		service:   mprisService,
		eventChan: make(chan EventData, 16),
		state:     &State{},
	}

	return s, nil
}

func (s *Service) Start() error {
	signalChan := make(chan *dbus.Signal, 10)
	s.signalChan = signalChan
	s.stopChan = make(chan struct{})

	s.bus.Signal(signalChan)

	matchPropertiesChanged := fmt.Sprintf(
		"type='signal',sender='%s',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='%s'",
		s.service, mprisPath,
	)
	matchSeeked := fmt.Sprintf(
		"type='signal',sender='%s',interface='%s',member='Seeked',path='%s'",
		s.service, mprisPlayerIface, mprisPath,
	)

	err := s.bus.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchPropertiesChanged).Err
	if err != nil {
		return fmt.Errorf("failed to add properties match: %w", err)
	}

	err = s.bus.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchSeeked).Err
	if err != nil {
		return fmt.Errorf("failed to add seeked match: %w", err)
	}

	go s.signalLoop()

	return nil
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		if s.stopChan != nil {
			close(s.stopChan)
		}
	})
}

func (s *Service) Events() <-chan EventData {
	return s.eventChan
}

func (s *Service) State() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Service) GetCurrentTrack() (*track.Info, error) {
	obj := s.bus.Object(s.service, mprisPath)
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

	info := &track.Info{
		Title:        extractString(metadata, "xesam:title"),
		Artist:       extractArtist(metadata, "xesam:artist"),
		Album:        extractString(metadata, "xesam:album"),
		ArtworkURL:   extractString(metadata, "mpris:artUrl"),
		TrackID:      extractString(metadata, "mpris:trackid"),
		DurationSecs: extractDurationSeconds(metadata, "mpris:length"),
	}

	if !info.IsValid() {
		return nil, fmt.Errorf("missing title or artist in metadata (title=%q, artist=%q)", info.Title, info.Artist)
	}

	return info, nil
}

func (s *Service) GetCurrentPosition() (int64, error) {
	obj := s.bus.Object(s.service, mprisPath)
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

func (s *Service) Poll() error {
	trk, err := s.GetCurrentTrack()
	if err != nil {
		return err
	}

	pos, err := s.GetCurrentPosition()
	if err != nil {
		return err
	}

	s.mu.Lock()
	currentTrack := s.state.Track
	seekDetected := s.state.DetectSeek(pos)
	s.state.UpdatePosition(pos)

	if !trk.IsSameTrack(currentTrack) {
		s.state.Track = trk
		s.mu.Unlock()
		s.emitEvent(EventData{Type: EventTrackChanged, Track: trk, Position: pos})
		return nil
	}
	s.mu.Unlock()

	if seekDetected {
		s.emitEvent(EventData{Type: EventSeeked, Position: pos})
	}

	return nil
}

func (s *Service) signalLoop() {
	for {
		select {
		case sig, ok := <-s.signalChan:
			if !ok {
				return
			}
			s.handleSignal(sig)
		case <-s.stopChan:
			return
		}
	}
}

func (s *Service) handleSignal(sig *dbus.Signal) {
	if sig == nil {
		return
	}

	switch sig.Name {
	case "org.freedesktop.DBus.Properties.PropertiesChanged":
		s.handlePropertiesChanged(sig)
	case "org.mpris.MediaPlayer2.Player.Seeked":
		s.handleSeeked(sig)
	}
}

func (s *Service) handlePropertiesChanged(sig *dbus.Signal) {
	if len(sig.Body) < 2 {
		return
	}

	interfaceName, ok := sig.Body[0].(string)
	if !ok || interfaceName != mprisPlayerIface {
		return
	}

	changedProps, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return
	}

	if metadataVariant, exists := changedProps["Metadata"]; exists {
		metadata, ok := metadataVariant.Value().(map[string]dbus.Variant)
		if !ok {
			return
		}

		info := &track.Info{
			Title:        extractString(metadata, "xesam:title"),
			Artist:       extractArtist(metadata, "xesam:artist"),
			Album:        extractString(metadata, "xesam:album"),
			ArtworkURL:   extractString(metadata, "mpris:artUrl"),
			TrackID:      extractString(metadata, "mpris:trackid"),
			DurationSecs: extractDurationSeconds(metadata, "mpris:length"),
		}

		if info.IsValid() {
			s.mu.Lock()
			s.state.Track = info
			s.state.lastPositionUpdate = time.Now()
			s.state.lastPositionSecs = 0
			s.mu.Unlock()

			s.emitEvent(EventData{Type: EventTrackChanged, Track: info})
		}
	}

	if playbackVariant, exists := changedProps["PlaybackStatus"]; exists {
		status, ok := playbackVariant.Value().(string)
		if ok {
			playing := status == "Playing"
			s.mu.Lock()
			s.state.Playing = playing
			s.state.lastPositionUpdate = time.Now()
			s.mu.Unlock()

			s.emitEvent(EventData{Type: EventPlaybackStateChanged, Playing: playing})
		}
	}
}

func (s *Service) handleSeeked(sig *dbus.Signal) {
	if len(sig.Body) < 1 {
		return
	}

	positionMicroseconds, ok := sig.Body[0].(int64)
	if !ok || positionMicroseconds < 0 {
		return
	}

	pos := positionMicroseconds / 1_000_000

	s.mu.Lock()
	s.state.UpdatePosition(pos)
	s.mu.Unlock()

	s.emitEvent(EventData{Type: EventSeeked, Position: pos})
}

func (s *Service) emitEvent(event EventData) {
	select {
	case s.eventChan <- event:
	default:
	}
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

func (s *Service) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// return a copy of the state
	stateCopy := State{
		PositionSecs: s.state.PositionSecs,
		Playing:      s.state.Playing,
	}

	// copy track info if it exists
	if s.state.Track != nil {
		trackCopy := *s.state.Track
		stateCopy.Track = &trackCopy
	}

	return stateCopy
}
