package poller

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
)

const storageBatch = 10

// fillStorageChat uploads the Tracks without a Telegram file, so inline
// mode sends them as audio. Whether a Track has one is no question for the
// core: the tables are the adapter's.
type fillStorageChat struct {
	files    *trackfile.Files
	chatID   int64
	musicDir string
}

func (s *fillStorageChat) Run(ctx context.Context) error {
	tracks, err := s.files.Unfiled(ctx, storageBatch)
	if err != nil {
		return err
	}
	for n := range tracks {
		track := &tracks[n]
		if err := s.files.PostUnlessBusy(ctx, s.chatID, &track.Track, filepath.Join(s.musicDir, track.Dir, track.Path)); err != nil {
			slog.Error("post_track_to_storage_chat", "track_id", track.ID, "error", err)
		}
	}
	return nil
}
