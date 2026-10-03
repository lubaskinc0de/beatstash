// Incoming and Probe: audio on its way into a Library, and the metadata
// chosen for it from the Provider's hints, the file's tags and its name.

package library

import (
	"path/filepath"
	"strings"

	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

// Incoming is a value object: audio being added to a Library, with the
// metadata chosen for it.
type Incoming struct {
	Ref          provider.TrackRef
	Metadata     Metadata
	Format       Format
	Quality      Quality
	DurationMs   int
	OriginalName string
}

// Probe is a value object: what the audio file itself says about the audio.
type Probe struct {
	Tags       Metadata
	DurationMs int
	Quality    Quality
}

// NewIncoming takes each metadata field from the first source that has
// it, in this order: the Provider's hint, the file's tags, the Provider's
// weak hint, and a file name like "Artist - Title".
func NewIncoming(ref provider.TrackRef, hint, weakHint Metadata, fileName string, format Format, probe Probe) Incoming {
	return Incoming{
		Ref:          ref,
		Metadata:     hint.Merge(probe.Tags, weakHint, metadataFromFileName(fileName)).Normalize(),
		Format:       format,
		Quality:      probe.Quality,
		DurationMs:   probe.DurationMs,
		OriginalName: fileName,
	}
}

func metadataFromFileName(name string) Metadata {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	artist, title, ok := strings.Cut(stem, " - ")
	if !ok {
		return Metadata{}
	}
	artist, title = collapseSpaces(artist), collapseSpaces(title)
	if artist == "" || title == "" {
		return Metadata{}
	}
	return Metadata{Artist: artist, Title: title}
}
