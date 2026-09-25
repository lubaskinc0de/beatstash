package libraries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain"
)

const sharedLibraryName = "Общая"

// MusicDir is music_dir as Navidrome sees it.
type Navidrome struct {
	Libraries *Libraries
	Navidrome navidrome.Client
	Admin     navidrome.Credentials
	MusicDir  string
}

// Grant lets the Navidrome Account see the user's Personal Library and the
// Shared Library, and nothing else.
func (n *Navidrome) Grant(ctx context.Context, user *domain.User, login string) error {
	libs, err := n.Libraries.Of(ctx, user)
	if err != nil {
		return err
	}
	for _, library := range []*domain.Library{libs.Personal, libs.Shared} {
		if err := n.Create(ctx, library); err != nil {
			return err
		}
	}
	return n.Navidrome.SetLibraries(ctx, n.Admin, login, []int{libs.Personal.NavidromeID, libs.Shared.NavidromeID})
}

// Create creates the library in Navidrome unless it is there
// already. The remembered id is checked first: an admin may have deleted
// the library in Navidrome, for one to let ND_MUSICFOLDER take its path.
func (n *Navidrome) Create(ctx context.Context, library *domain.Library) error {
	existing, err := n.Navidrome.Libraries(ctx, n.Admin)
	if err != nil {
		return err
	}
	if library.NavidromeID != 0 && slices.ContainsFunc(existing, func(nd navidrome.Library) bool {
		return nd.ID == library.NavidromeID
	}) {
		return nil
	}

	id := n.adopt(library, existing)
	if id == 0 {
		if id, err = n.create(ctx, library); err != nil {
			return err
		}
	}

	if err := n.Libraries.Repo.SetNavidromeID(ctx, library.ID, id); err != nil {
		return err
	}
	library.NavidromeID = id
	return nil
}

// ShowNewAccountsOnlyShared keeps an account that Navidrome creates, or one
// the bot failed to grant, away from Personal Libraries: of the libraries
// under music_dir only the Shared Library goes to new accounts.
func (n *Navidrome) ShowNewAccountsOnlyShared(ctx context.Context) error {
	shared, err := n.Libraries.Shared(ctx)
	if err != nil {
		return err
	}
	existing, err := n.Navidrome.Libraries(ctx, n.Admin)
	if err != nil {
		return err
	}

	root := path.Clean(n.MusicDir)
	for _, nd := range existing {
		if nd.Path != root && !strings.HasPrefix(nd.Path, root+"/") {
			continue
		}
		wanted := nd.ID == shared.NavidromeID
		if nd.DefaultNewUsers == wanted {
			continue
		}
		nd.DefaultNewUsers = wanted
		if err := n.Navidrome.UpdateLibrary(ctx, n.Admin, nd); err != nil {
			return err
		}
	}
	return nil
}

// adopt finds the library among Navidrome's by path.
func (n *Navidrome) adopt(library *domain.Library, existing []navidrome.Library) int {
	target := n.navidromePath(library)
	for _, nd := range existing {
		if nd.Path == target {
			return nd.ID
		}
	}

	root := path.Clean(n.MusicDir)
	if library.Kind == domain.LibraryShared && slices.ContainsFunc(existing, func(nd navidrome.Library) bool {
		return nd.ID == navidromeRootLibraryID && nd.Path == root
	}) {
		slog.Warn(
			"navidrome_root_library_spans_music_dir",
			"hint", fmt.Sprintf(
				"set ND_MUSICFOLDER to %s, delete the bot's Shared Library in navidrome.Client, restart navidrome.Client, then the bot; "+
					"until then admins see every track twice",
				target,
			),
		)
	}
	return 0
}

// create falls back to a name with the path in it: names are unique across
// Navidrome, which may hold libraries of other bots.
func (n *Navidrome) create(ctx context.Context, library *domain.Library) (int, error) {
	nd := navidrome.Library{
		Name:            libraryName(library),
		Path:            n.navidromePath(library),
		DefaultNewUsers: library.Kind == domain.LibraryShared,
	}
	id, err := n.Navidrome.CreateLibrary(ctx, n.Admin, nd)
	if errors.Is(err, navidrome.ErrNameTaken) {
		nd.Name = fallbackName(nd)
		id, err = n.Navidrome.CreateLibrary(ctx, n.Admin, nd)
	}
	return id, err
}

func (n *Navidrome) navidromePath(library *domain.Library) string {
	return path.Join(n.MusicDir, filepath.ToSlash(library.Dir))
}

// navidromeRootLibraryID is the library Navidrome makes from ND_MUSICFOLDER;
// its path follows that setting and can be neither changed nor deleted.
const navidromeRootLibraryID = 1

func fallbackName(nd navidrome.Library) string {
	return nd.Name + " · " + nd.Path
}

func libraryName(library *domain.Library) string {
	if library.Kind == domain.LibraryShared {
		return sharedLibraryName
	}
	return "Личная · " + filepath.Base(library.Dir)
}
