package audio

import (
	"fmt"
	"strconv"
	"strings"

	"go.senan.xyz/taglib"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

type Tags struct{}

func (Tags) Probe(path string) (*library.Probe, error) {
	props, err := taglib.ReadProperties(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ingest_track.ErrCorruptAudio, err)
	}
	if props.Length <= 0 {
		return nil, fmt.Errorf("%w: no duration", ingest_track.ErrCorruptAudio)
	}

	tags, err := taglib.ReadTags(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ingest_track.ErrCorruptAudio, err)
	}

	return &library.Probe{
		Tags:       metadataFromTags(tags),
		DurationMs: int(props.Length.Milliseconds()),
		Quality: library.Quality{
			Lossless: isLossless(props),
			Bitrate:  int(props.BitRate),
		},
	}, nil
}

func isLossless(props taglib.Properties) bool {
	switch props.Format {
	case "flac", "wav":
		return true
	case "mp4":
		return props.InnerCodec == "alac"
	}
	return false
}

func metadataFromTags(tags map[string][]string) library.Metadata {
	first := func(key string) string {
		if values := tags[key]; len(values) > 0 {
			return values[0]
		}
		return ""
	}

	return library.Metadata{
		AlbumArtist: first(taglib.AlbumArtist),
		Artist:      first(taglib.Artist),
		Album:       first(taglib.Album),
		Title:       first(taglib.Title),
		Year:        leadingNumber(first(taglib.Date)),
		TrackNumber: leadingNumber(first(taglib.TrackNumber)),
	}
}

// leadingNumber reads "2001-05-01" as 2001 and "3/12" as 3.
func leadingNumber(s string) int {
	s = strings.TrimSpace(s)
	end := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if end >= 0 {
		s = s[:end]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func (Tags) WriteTags(path string, m library.Metadata) error {
	tags := map[string][]string{}
	set := func(key, value string) {
		if value != "" {
			tags[key] = []string{value}
		}
	}
	set(taglib.AlbumArtist, m.AlbumArtist)
	set(taglib.Artist, m.Artist)
	set(taglib.Album, m.Album)
	set(taglib.Title, m.Title)
	if m.Year > 0 {
		set(taglib.Date, strconv.Itoa(m.Year))
	}
	if m.TrackNumber > 0 {
		set(taglib.TrackNumber, strconv.Itoa(m.TrackNumber))
	}

	if err := taglib.WriteTags(path, tags, 0); err != nil {
		return fmt.Errorf("write tags: %w", err)
	}
	return nil
}

func (Tags) WriteCover(path string, image []byte) error {
	if err := taglib.WriteImage(path, image); err != nil {
		return fmt.Errorf("write cover: %w", err)
	}
	return nil
}
