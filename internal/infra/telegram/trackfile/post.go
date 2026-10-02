package trackfile

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// Post is a Track's file on its way to Telegram; a successful post leaves a
// File instead.
type Post struct {
	TrackID uint `gorm:"primaryKey;autoIncrement:false"`
	// ClaimedUntil is the lease of whoever posts it now; its value is
	// their token.
	ClaimedUntil *time.Time
	RejectedSize *int64
}

func (Post) TableName() string {
	return "telegram_file_posts"
}

// postWait is how often a request waiting for another's post looks for the
// file.
const postWait = 100 * time.Millisecond

// For posts the Track to the chat unless it has a file already; posted
// tells which. While somebody else posts it, For waits for their file.
func (f *Files) For(ctx context.Context, chatID int64, track *library.Track, path string) (file *File, posted bool, err error) {
	for {
		file, err := f.stored(ctx, track)
		if err != nil || file != nil {
			return file, false, err
		}
		token, err := f.claim(ctx, track.ID)
		if err != nil {
			return nil, false, err
		}
		if token != nil {
			return f.post(ctx, chatID, track, path, *token)
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(postWait):
		}
	}
}

// SendTo posts the Track only if it has no file yet.
func (f *Files) SendTo(ctx context.Context, chatID int64, track *library.Track, path string) error {
	file, posted, err := f.For(ctx, chatID, track, path)
	if err != nil || posted {
		return err
	}
	return f.Sender.Send(ctx, chatID, file)
}

// PostUnlessBusy posts the Track to the chat, unless somebody else is
// posting it now: it does not wait for them.
func (f *Files) PostUnlessBusy(ctx context.Context, chatID int64, track *library.Track, path string) error {
	token, err := f.claim(ctx, track.ID)
	if err != nil || token == nil {
		return err
	}
	_, _, err = f.post(ctx, chatID, track, path, *token)
	return err
}

func (f *Files) post(ctx context.Context, chatID int64, track *library.Track, path string, token time.Time) (*File, bool, error) {
	// The previous claimant may have finished between the look and the claim.
	if file, err := f.stored(ctx, track); err != nil || file != nil {
		f.release(ctx, track.ID, token, nil)
		return file, false, err
	}
	posted, err := f.Sender.post(ctx, chatID, path, track)
	if err != nil {
		var tooLarge *TooLargeError
		if errors.As(err, &tooLarge) {
			f.release(ctx, track.ID, token, &tooLarge.Size)
		} else {
			f.release(ctx, track.ID, token, nil)
		}
		return nil, false, err
	}
	// The file serves the post that made it even if it is not remembered.
	if err := f.Remember(ctx, *posted); err != nil {
		slog.Error("remember_telegram_file", "track_id", track.ID, "error", err)
	}
	err = f.DB.WithContext(ctx).Where("track_id = ? AND claimed_until = ?", track.ID, token).Delete(&Post{}).Error
	if err != nil {
		slog.Error("forget_telegram_post", "track_id", track.ID, "error", err)
	}
	return posted, true, nil
}

func (f *Files) stored(ctx context.Context, track *library.Track) (*File, error) {
	files, err := f.Of(ctx, []uint{track.ID})
	if err != nil {
		return nil, err
	}
	return files[track.ID], nil
}

// claim returns the lease on the Track's post, or nil if somebody else
// holds it.
func (f *Files) claim(ctx context.Context, trackID uint) (*time.Time, error) {
	var claimed []time.Time
	err := f.DB.WithContext(ctx).Raw(`
		INSERT INTO telegram_file_posts (track_id, claimed_until) VALUES (@track, now() + @ttl * interval '1 millisecond')
		ON CONFLICT (track_id) DO UPDATE SET claimed_until = EXCLUDED.claimed_until
		WHERE telegram_file_posts.claimed_until IS NULL OR telegram_file_posts.claimed_until < now()
		RETURNING claimed_until`,
		sql.Named("track", trackID), sql.Named("ttl", f.LeaseTTL.Milliseconds()),
	).Scan(&claimed).Error
	if err != nil || len(claimed) == 0 {
		return nil, err
	}
	return &claimed[0], nil
}

// release keeps what Telegram would not take, if anything: the next
// post waits for a higher limit. It frees the post even for an instance
// that is stopping.
func (f *Files) release(ctx context.Context, trackID uint, token time.Time, rejectedSize *int64) {
	freed := map[string]any{"claimed_until": nil}
	if rejectedSize != nil {
		freed["rejected_size"] = *rejectedSize
	}
	err := f.DB.WithContext(context.WithoutCancel(ctx)).Model(&Post{}).
		Where("track_id = ? AND claimed_until = ?", trackID, token).
		Updates(freed).Error
	if err != nil {
		slog.Error("release_telegram_post", "track_id", trackID, "error", err)
	}
}
