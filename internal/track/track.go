package track

type Info struct {
	Title        string
	Artist       string
	Album        string
	DurationSecs int64
	ArtworkURL   string
	TrackID      string
}

func (t *Info) IsValid() bool {
	if t == nil {
		return false
	}
	return t.Title != "" && t.Artist != ""
}

func (t *Info) IsSameTrack(other *Info) bool {
	if t == nil || other == nil {
		return t == other
	}
	if t.TrackID != "" && other.TrackID != "" {
		return t.TrackID == other.TrackID
	}
	return t.Title == other.Title && t.Artist == other.Artist
}

