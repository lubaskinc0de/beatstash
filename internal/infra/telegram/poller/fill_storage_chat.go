package poller

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/trackfile"
)

const storageBatch = 10

// fillStorageChat uploads the Tracks without a Telegram file, so inline
// mode sends them as audio. Whether a Track has one is no question for the
// core: the tables are the adapter's.
type fillStorageChat struct {
	files             *trackfile.Files
	chatID            int64
	musicDir          string
	navidromeMusicDir string
}

func (s *fillStorageChat) Run(ctx context.Context) error {
	tracks, err := s.files.Unfiled(ctx, storageBatch)
	if err != nil {
		return err
	}
	for n := range tracks {
		track := &tracks[n]
		path, ok := s.pathOf(track)
		if !ok {
			slog.Error("post_track_file_is_not_reachable", "track_id", track.ID, "path", path)
			if err := s.files.MarkUnreachable(ctx, track.ID); err != nil {
				slog.Error("mark_track_file_unreachable", "track_id", track.ID, "error", err)
			}
			continue
		}
		if err := s.files.PostUnlessBusy(ctx, s.chatID, &track.Track, path); err != nil {
			slog.Error("post_track_to_storage_chat", "track_id", track.ID, "error", err)
		}
	}
	return nil
}

// pathOf finds the Track's file on the bot's disk. An Attached Library's dir
// is where Navidrome sees it, and the bot reads only the part under
// music_dir.
func (s *fillStorageChat) pathOf(track *trackfile.Unfiled) (string, bool) {
	dir := track.Dir
	if track.Attached() {
		rel, err := filepath.Rel(s.navidromeMusicDir, track.Dir)
		if err != nil || !filepath.IsLocal(rel) {
			return filepath.Join(track.Dir, track.Path), false
		}
		dir = rel
	}
	path := filepath.Join(s.musicDir, dir, track.Path)
	_, err := os.Stat(path)
	return path, err == nil
}
