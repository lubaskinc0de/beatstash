package attach_libraries

import (
	"context"
	"errors"
	"log/slog"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// AttachLibraries takes every Navidrome library outside music_dir into
// account as an Attached Library and brings its Tracks in step with the
// songs Navidrome has indexed there. A library gone from Navidrome, and its
// Tracks, go too.
type AttachLibraries struct {
	Tx        repositories.TxManager
	Libraries repositories.Libraries
	// Lock keeps Ingest from giving a Source to a Track this removes.
	Lock      repositories.LibraryLock
	Tracks    repositories.Tracks
	Navidrome navidrome.Client
	Admin     navidrome.Credentials
	MusicDir  string
}

func (i *AttachLibraries) Execute(ctx context.Context) error {
	existing, err := i.Navidrome.Libraries(ctx, i.Admin)
	if err != nil {
		return err
	}
	known, err := i.Libraries.Attached(ctx)
	if err != nil {
		return err
	}

	attachable := map[int]navidrome.Library{}
	for _, nd := range existing {
		switch library.PlacementOf(nd.Path, i.MusicDir) {
		case library.PlacedInside:
		case library.PlacedAround:
			slog.Warn("navidrome_library_spans_music_dir", "library", nd.Name, "path", nd.Path)
		default:
			attachable[nd.ID] = nd
		}
	}

	byNavidromeID := map[int]*library.Library{}
	var gone []uint
	for n := range known {
		lib := &known[n]
		if _, ok := attachable[lib.NavidromeID]; ok {
			byNavidromeID[lib.NavidromeID] = lib
		} else {
			gone = append(gone, lib.ID)
		}
	}
	// Gone first: a new library may take the path of a gone one.
	if err := i.detach(ctx, gone); err != nil {
		return err
	}

	var errs []error
	for id, nd := range attachable {
		if err := i.attach(ctx, byNavidromeID[id], nd); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// detach takes the libraries gone from Navidrome off the bot's books, with
// their Tracks.
func (i *AttachLibraries) detach(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := i.Lock.Lock(ctx, ids...); err != nil {
			return err
		}
		return i.Libraries.Delete(ctx, ids)
	})
}

// attach takes the library's songs; lib is nil for a library new to the bot.
func (i *AttachLibraries) attach(ctx context.Context, lib *library.Library, nd navidrome.Library) error {
	songs, err := i.Navidrome.LibrarySongs(ctx, i.Admin, nd.ID)
	if err != nil {
		return err
	}
	return i.Tx.WithinTx(ctx, func(ctx context.Context) error {
		switch {
		case lib == nil:
			lib = library.AttachedLibrary(nd.ID, nd.Path)
		case lib.Dir != nd.Path:
			lib.MoveAttached(nd.Path)
		}
		if err := i.Libraries.Save(ctx, lib); err != nil {
			return err
		}
		if err := i.Lock.Lock(ctx, lib.ID); err != nil {
			return err
		}

		tracks, err := i.Tracks.InLibrary(ctx, lib.ID)
		if err != nil {
			return err
		}
		indexed := make([]library.Song, 0, len(songs))
		for _, song := range songs {
			indexed = append(indexed, songOf(song))
		}
		save, gone := lib.Follow(tracks, indexed)

		ids := make([]uint, 0, len(gone))
		for _, track := range gone {
			ids = append(ids, track.ID)
		}
		// Gone first: a new song may take the path of a gone one.
		if err := i.Tracks.Delete(ctx, ids); err != nil {
			return err
		}
		return i.Tracks.SaveTracks(ctx, save)
	})
}

func songOf(s navidrome.Song) library.Song {
	format, ok := library.FormatOf("."+s.Suffix, "")
	if !ok {
		format = library.Format(s.Suffix)
	}
	return library.Song{
		ID:   s.ID,
		Path: s.Path,
		Metadata: library.Metadata{
			AlbumArtist: s.AlbumArtist,
			Artist:      s.Artist,
			Album:       s.Album,
			Title:       s.Title,
			Year:        s.Year,
			TrackNumber: s.TrackNumber,
		},
		DurationMs: s.DurationMs,
		Format:     format,
		Quality: library.Quality{
			Lossless: library.LosslessCodec(s.Codec) || library.LosslessCodec(s.Suffix),
			Bitrate:  s.BitrateKbps,
		},
	}
}
