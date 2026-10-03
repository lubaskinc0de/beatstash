package libraries

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"slices"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

const sharedLibraryName = "Общая"

type Navidrome struct {
	Repo      repositories.Libraries
	Navidrome navidrome.Client
	Admin     navidrome.Credentials
	Folders   library.SystemFolders
}

// Grant creates the user's libraries in Navidrome and gives the account the
// access library.Grant decides; it returns what the account sees then.
func (n *Navidrome) Grant(ctx context.Context, user *access.User, login string) (library.NavidromeAccess, error) {
	personal, err := n.Repo.Personal(ctx, user.ID)
	if err != nil {
		return library.NavidromeAccess{}, err
	}
	shared, err := n.Repo.Shared(ctx)
	if err != nil {
		return library.NavidromeAccess{}, err
	}
	for _, lib := range []*library.Library{personal, shared} {
		if err := n.Create(ctx, lib); err != nil {
			return library.NavidromeAccess{}, err
		}
	}
	account, err := n.Navidrome.Account(ctx, n.Admin, login)
	if err != nil {
		return library.NavidromeAccess{}, err
	}
	if account.Access.Admin {
		return account.Access, navidrome.ErrAdminAccount
	}
	all, err := n.Repo.All(ctx)
	if err != nil {
		return library.NavidromeAccess{}, err
	}
	existing, err := n.Navidrome.Libraries(ctx, n.Admin)
	if err != nil {
		return library.NavidromeAccess{}, err
	}
	paths := make(map[int]string, len(existing))
	for _, nd := range existing {
		paths[nd.ID] = nd.Path
	}
	ids := library.Grant(account.Access.LibraryIDs, all, paths, n.Folders, personal, shared)
	if err := n.Navidrome.SetLibraries(ctx, n.Admin, account.ID, ids); err != nil {
		return library.NavidromeAccess{}, err
	}
	return library.NavidromeAccess{LibraryIDs: ids}, nil
}

// Create creates the library in Navidrome unless it is there
// already. The remembered id is checked first: an admin may have deleted
// the library in Navidrome, for one to let ND_MUSICFOLDER take its path.
func (n *Navidrome) Create(ctx context.Context, lib *library.Library) error {
	existing, err := n.Navidrome.Libraries(ctx, n.Admin)
	if err != nil {
		return err
	}
	if lib.NavidromeID != 0 && slices.ContainsFunc(existing, func(nd navidrome.Library) bool {
		return nd.ID == lib.NavidromeID
	}) {
		return nil
	}

	id := n.adopt(lib, existing)
	if id == 0 {
		if id, err = n.create(ctx, lib); err != nil {
			return err
		}
	}

	lib.LinkNavidrome(id)
	return n.Repo.Save(ctx, lib)
}

// ShowNewAccountsOnlyShared keeps an account that Navidrome creates, or one
// the bot failed to grant, away from Personal Libraries: of the libraries
// in or around the bot's folders only the Shared Library goes to new
// accounts. Attached Libraries are the admin's to set.
func (n *Navidrome) ShowNewAccountsOnlyShared(ctx context.Context) error {
	shared, err := n.Repo.Shared(ctx)
	if err != nil {
		return err
	}
	existing, err := n.Navidrome.Libraries(ctx, n.Admin)
	if err != nil {
		return err
	}

	for _, nd := range existing {
		if library.PlacementOf(nd.Path, n.Folders) == library.PlacedApart {
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
func (n *Navidrome) adopt(lib *library.Library, existing []navidrome.Library) int {
	target := n.navidromePath(lib)
	for _, nd := range existing {
		if nd.Path == target {
			return nd.ID
		}
	}

	root := path.Clean(n.Folders.Root)
	if lib.Kind == library.LibraryShared && slices.ContainsFunc(existing, func(nd navidrome.Library) bool {
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
func (n *Navidrome) create(ctx context.Context, lib *library.Library) (int, error) {
	nd := navidrome.Library{
		Name:            libraryName(lib),
		Path:            n.navidromePath(lib),
		DefaultNewUsers: lib.Kind == library.LibraryShared,
	}
	id, err := n.Navidrome.CreateLibrary(ctx, n.Admin, nd)
	if errors.Is(err, navidrome.ErrNameTaken) {
		nd.Name = fallbackName(nd)
		id, err = n.Navidrome.CreateLibrary(ctx, n.Admin, nd)
	}
	return id, err
}

func (n *Navidrome) navidromePath(lib *library.Library) string {
	return path.Join(n.Folders.Root, filepath.ToSlash(lib.Dir))
}

// navidromeRootLibraryID is the library Navidrome makes from ND_MUSICFOLDER;
// its path follows that setting and can be neither changed nor deleted.
const navidromeRootLibraryID = 1

func fallbackName(nd navidrome.Library) string {
	return nd.Name + " · " + nd.Path
}

func libraryName(lib *library.Library) string {
	if lib.Kind == library.LibraryShared {
		return sharedLibraryName
	}
	return "Личная · " + filepath.Base(lib.Dir)
}
