package audiotag

import (
	"io"

	"github.com/dhowden/tag"
)

// Result holds music/audio tag metadata.
type Result struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Track       int
	Year        int
}

// Extract reads audio tags (ID3v2, Vorbis, MP4 atoms) from the reader.
// Returns nil if no tags are found or the format is unsupported.
func Extract(r io.ReadSeeker) *Result {
	m, err := tag.ReadFrom(r)
	if err != nil {
		return nil
	}

	track, _ := m.Track()

	return &Result{
		Title:       m.Title(),
		Artist:      m.Artist(),
		Album:       m.Album(),
		AlbumArtist: m.AlbumArtist(),
		Genre:       m.Genre(),
		Track:       track,
		Year:        m.Year(),
	}
}
