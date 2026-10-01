package common

import (
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type AudioTags interface {
	// Probe returns ErrCorruptAudio for unreadable tags or no duration.
	Probe(path string) (*library.Probe, error)
	WriteTags(path string, m library.Metadata) error
	WriteCover(path string, image []byte) error
}

var ErrCorruptAudio = errors.New("audio file is corrupt")
