package libraries

import (
	"context"
	"path/filepath"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

// Libraries creates a library's row and directory on first use.
type Libraries struct {
	Repo     repositories.Libraries
	Disk     common.Disk
	MusicDir string
}

func (l *Libraries) Shared(ctx context.Context) (*domain.Library, error) {
	return l.ensure(ctx, domain.SharedLibrary())
}

func (l *Libraries) Personal(ctx context.Context, owner *domain.User) (*domain.Library, error) {
	return l.ensure(ctx, domain.PersonalLibrary(owner))
}

func (l *Libraries) ensure(ctx context.Context, library *domain.Library) (*domain.Library, error) {
	if err := l.Disk.MakeDir(l.Dir(library)); err != nil {
		return nil, err
	}
	if err := l.Repo.Ensure(ctx, library); err != nil {
		return nil, err
	}
	return library, nil
}

func (l *Libraries) Dir(library *domain.Library) string {
	return Dir(l.MusicDir, library)
}

func (l *Libraries) FilePath(ctx context.Context, track *domain.Track) (string, error) {
	return TrackPath(ctx, l.Repo, l.MusicDir, track)
}

// Of returns the user's Personal Library and the Shared Library, creating
// either one (row and directory) if it does not exist yet.
func (l *Libraries) Of(ctx context.Context, user *domain.User) (UserLibraries, error) {
	personal, err := l.Personal(ctx, user)
	if err != nil {
		return UserLibraries{}, err
	}
	shared, err := l.Shared(ctx)
	if err != nil {
		return UserLibraries{}, err
	}
	return UserLibraries{Personal: personal, Shared: shared}, nil
}

func Dir(musicDir string, library *domain.Library) string {
	return filepath.Join(musicDir, library.Dir)
}

func TrackPath(ctx context.Context, libraries repositories.Libraries, musicDir string, track *domain.Track) (string, error) {
	library, err := libraries.Get(ctx, track.LibraryID)
	if err != nil {
		return "", err
	}
	return filepath.Join(Dir(musicDir, library), track.Path), nil
}

// UserLibraries are the libraries a user sees: their own and the shared one.
type UserLibraries struct {
	Personal *domain.Library
	Shared   *domain.Library
}

func (u UserLibraries) IDs() []uint {
	return []uint{u.Personal.ID, u.Shared.ID}
}

func Current(ctx context.Context, ids common.IDProvider, libraries *Libraries) (*domain.User, UserLibraries, error) {
	user, err := ids.CurrentUser(ctx)
	if err != nil {
		return nil, UserLibraries{}, err
	}
	libs, err := libraries.Of(ctx, user)
	if err != nil {
		return nil, UserLibraries{}, err
	}
	return user, libs, nil
}
