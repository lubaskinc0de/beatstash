package libraries

import (
	"context"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

// Libraries are created, row and directory, when the server starts and when
// a User joins; every other request finds them there.
type Libraries struct {
	Repo     repositories.Libraries
	Disk     common.Disk
	MusicDir string
}

func (l *Libraries) EnsureShared(ctx context.Context) (*library.Library, error) {
	return l.ensure(ctx, library.SharedLibrary())
}

func (l *Libraries) EnsurePersonal(ctx context.Context, owner *access.User) (*library.Library, error) {
	return l.ensure(ctx, library.PersonalLibrary(owner))
}

func (l *Libraries) ensure(ctx context.Context, lib *library.Library) (*library.Library, error) {
	if err := l.Disk.MakeDir(Dir(l.MusicDir, lib)); err != nil {
		return nil, err
	}
	if err := l.Repo.Ensure(ctx, lib); err != nil {
		return nil, err
	}
	return lib, nil
}

func (l *Libraries) FilePath(ctx context.Context, track *library.Track) (string, error) {
	return TrackPath(ctx, l.Repo, l.MusicDir, track)
}

func (l *Libraries) Of(ctx context.Context, user *access.User) (ManagedLibraries, error) {
	personal, err := l.Repo.Personal(ctx, user.ID)
	if err != nil {
		return ManagedLibraries{}, err
	}
	shared, err := l.Repo.Shared(ctx)
	if err != nil {
		return ManagedLibraries{}, err
	}
	return ManagedLibraries{Personal: personal, Shared: shared}, nil
}

func Dir(musicDir string, lib *library.Library) string {
	return filepath.Join(musicDir, lib.Dir)
}

func TrackPath(ctx context.Context, libraries repositories.Libraries, musicDir string, track *library.Track) (string, error) {
	lib, err := libraries.Get(ctx, track.LibraryID)
	if err != nil {
		return "", err
	}
	return filepath.Join(Dir(musicDir, lib), track.Path), nil
}

// ManagedLibraries are the libraries the bot keeps and writes to for a user:
// their Personal Library and the Shared Library.
type ManagedLibraries struct {
	Personal *library.Library
	Shared   *library.Library
}

func (u ManagedLibraries) IDs() []uint {
	return []uint{u.Personal.ID, u.Shared.ID}
}

func CurrentKept(
	ctx context.Context,
	ids common.IDProvider,
	libraries *Libraries,
	attached *Attached,
) (*access.User, ManagedLibraries, []*library.Library, error) {
	user, err := ids.CurrentUser(ctx)
	if err != nil {
		return nil, ManagedLibraries{}, nil, err
	}
	libs, err := libraries.Of(ctx, user)
	if err != nil {
		return nil, ManagedLibraries{}, nil, err
	}
	visible, err := attached.VisibleTo(ctx, user.ID)
	if err != nil {
		return nil, ManagedLibraries{}, nil, err
	}
	return user, libs, library.KeptLibraries(libs.Personal, visible), nil
}
