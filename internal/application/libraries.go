package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

const sharedLibraryName = "Общая"

// Libraries keeps Personal and Shared Libraries in step across the
// database, music_dir and Navidrome.
type Libraries struct {
	Repo      LibraryRepository
	Users     UserLister
	Accounts  NavidromeAccountLister
	Navidrome Navidrome
	Admin     NavidromeCredentials

	MusicDir          string
	NavidromeMusicDir string
}

type UserLister interface {
	All(ctx context.Context) ([]domain.User, error)
}

type NavidromeAccountLister interface {
	All(ctx context.Context) ([]domain.NavidromeAccount, error)
}

func (l *Libraries) Shared(ctx context.Context) (*domain.Library, error) {
	return l.ensure(ctx, domain.SharedLibrary())
}

func (l *Libraries) Personal(ctx context.Context, owner *domain.User) (*domain.Library, error) {
	return l.ensure(ctx, domain.PersonalLibrary(owner))
}

func (l *Libraries) ensure(ctx context.Context, library *domain.Library) (*domain.Library, error) {
	if err := os.MkdirAll(l.Dir(library), 0o755); err != nil {
		return nil, err
	}
	if err := l.Repo.Ensure(ctx, library); err != nil {
		return nil, err
	}
	return library, nil
}

func (l *Libraries) Dir(library *domain.Library) string {
	return filepath.Join(l.MusicDir, library.Dir)
}

// UserLibraries are the libraries a user sees: their own and the shared one.
type UserLibraries struct {
	Personal *domain.Library
	Shared   *domain.Library
}

func (u UserLibraries) IDs() []uint {
	return []uint{u.Personal.ID, u.Shared.ID}
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

// Prepare runs on startup: every User gets a Personal Library, every
// library exists in Navidrome, and every Navidrome Account sees exactly its
// own and the Shared Library. Navidrome being down is not fatal: the next
// start or grant finishes the job.
func (l *Libraries) Prepare(ctx context.Context) error {
	if _, err := l.Shared(ctx); err != nil {
		return err
	}
	users, err := l.Users.All(ctx)
	if err != nil {
		return err
	}
	byID := make(map[uint]*domain.User, len(users))
	for i := range users {
		byID[users[i].ID] = &users[i]
		if _, err := l.Personal(ctx, &users[i]); err != nil {
			return err
		}
	}

	accounts, err := l.Accounts.All(ctx)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		err := l.Grant(ctx, byID[account.UserID], account.Login)
		if err != nil && !errors.Is(err, ErrNavidromeAdminAccount) {
			slog.Error("grant_navidrome_libraries", "user_id", account.UserID, "error", err)
		}
	}

	libraries, err := l.Repo.All(ctx)
	if err != nil {
		return err
	}
	for i := range libraries {
		if err := l.CreateInNavidrome(ctx, &libraries[i]); err != nil {
			slog.Error("create_library_in_navidrome", "dir", libraries[i].Dir, "error", err)
		}
	}
	if err := l.showNewAccountsOnlyShared(ctx); err != nil {
		slog.Error("set_navidrome_default_libraries", "error", err)
	}
	return nil
}

// showNewAccountsOnlyShared keeps an account that Navidrome creates, or one
// the bot failed to grant, away from Personal Libraries: of the libraries
// under music_dir only the Shared Library goes to new accounts.
func (l *Libraries) showNewAccountsOnlyShared(ctx context.Context) error {
	shared, err := l.Shared(ctx)
	if err != nil {
		return err
	}
	existing, err := l.Navidrome.Libraries(ctx, l.Admin)
	if err != nil {
		return err
	}

	root := path.Clean(l.NavidromeMusicDir)
	for _, nd := range existing {
		if nd.Path != root && !strings.HasPrefix(nd.Path, root+"/") {
			continue
		}
		wanted := nd.ID == shared.NavidromeID
		if nd.DefaultNewUsers == wanted {
			continue
		}
		nd.DefaultNewUsers = wanted
		if err := l.Navidrome.UpdateLibrary(ctx, l.Admin, nd); err != nil {
			return err
		}
	}
	return nil
}

// Grant lets the Navidrome Account see the user's Personal Library and the
// Shared Library, and nothing else.
func (l *Libraries) Grant(ctx context.Context, user *domain.User, login string) error {
	libs, err := l.Of(ctx, user)
	if err != nil {
		return err
	}
	for _, library := range []*domain.Library{libs.Personal, libs.Shared} {
		if err := l.CreateInNavidrome(ctx, library); err != nil {
			return err
		}
	}
	return l.Navidrome.SetLibraries(ctx, l.Admin, login, []int{libs.Personal.NavidromeID, libs.Shared.NavidromeID})
}

// CreateInNavidrome creates the library in Navidrome unless it is there
// already. The remembered id is checked first: an admin may have deleted
// the library in Navidrome, for one to let ND_MUSICFOLDER take its path.
func (l *Libraries) CreateInNavidrome(ctx context.Context, library *domain.Library) error {
	existing, err := l.Navidrome.Libraries(ctx, l.Admin)
	if err != nil {
		return err
	}
	if library.NavidromeID != 0 && slices.ContainsFunc(existing, func(nd NavidromeLibrary) bool {
		return nd.ID == library.NavidromeID
	}) {
		return nil
	}

	id := l.adopt(library, existing)
	if id == 0 {
		if id, err = l.create(ctx, library); err != nil {
			return err
		}
	}

	if err := l.Repo.SetNavidromeID(ctx, library.ID, id); err != nil {
		return err
	}
	library.NavidromeID = id
	return nil
}

// navidromeRootLibraryID is the library Navidrome makes from ND_MUSICFOLDER;
// its path follows that setting and can be neither changed nor deleted.
const navidromeRootLibraryID = 1

// adopt finds the library among Navidrome's by path.
func (l *Libraries) adopt(library *domain.Library, existing []NavidromeLibrary) int {
	target := l.navidromePath(library)
	for _, nd := range existing {
		if nd.Path == target {
			return nd.ID
		}
	}

	root := path.Clean(l.NavidromeMusicDir)
	if library.Kind == domain.LibraryShared && slices.ContainsFunc(existing, func(nd NavidromeLibrary) bool {
		return nd.ID == navidromeRootLibraryID && nd.Path == root
	}) {
		slog.Warn(
			"navidrome_root_library_spans_music_dir",
			"hint", fmt.Sprintf(
				"set ND_MUSICFOLDER to %s, delete the bot's Shared Library in Navidrome, restart Navidrome, then the bot; "+
					"until then admins see every track twice",
				target,
			),
		)
	}
	return 0
}

// create falls back to a name with the path in it: names are unique across
// Navidrome, which may hold libraries of other bots.
func (l *Libraries) create(ctx context.Context, library *domain.Library) (int, error) {
	nd := NavidromeLibrary{
		Name:            libraryName(library),
		Path:            l.navidromePath(library),
		DefaultNewUsers: library.Kind == domain.LibraryShared,
	}
	id, err := l.Navidrome.CreateLibrary(ctx, l.Admin, nd)
	if errors.Is(err, ErrNavidromeNameTaken) {
		nd.Name = fallbackName(nd)
		id, err = l.Navidrome.CreateLibrary(ctx, l.Admin, nd)
	}
	return id, err
}

func fallbackName(nd NavidromeLibrary) string {
	return nd.Name + " · " + nd.Path
}

func libraryName(library *domain.Library) string {
	if library.Kind == domain.LibraryShared {
		return sharedLibraryName
	}
	return "Личная · " + filepath.Base(library.Dir)
}

func (l *Libraries) navidromePath(library *domain.Library) string {
	return path.Join(l.NavidromeMusicDir, filepath.ToSlash(library.Dir))
}
