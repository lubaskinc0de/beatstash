package harness

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/disk"
)

func (s *Scenario) LinkNewAccount(user User) navidrome.Account {
	s.t.Helper()

	account := s.Navidrome.CreateAccount(s.t, user.Username)
	s.Link(user, account)
	require.Contains(s.t, s.WindowText(), s.Catalog(user).Linked(account.Login, 0))
	return account
}

func (s *Scenario) NavidromePath(rel string) string {
	return filepath.Join(s.config.NavidromeMusicDir, rel)
}

var (
	issuedLogin    = regexp.MustCompile(`Логин: <code>([^<]+)</code>`)
	issuedPassword = regexp.MustCompile(`<tg-spoiler>([^<]+)</tg-spoiler>`)
)

// IssuedAccount reads the credentials the bot showed in its last reply.
func (s *Scenario) IssuedAccount() navidrome.Account {
	s.t.Helper()

	text := s.LastReply().Text
	login := issuedLogin.FindStringSubmatch(text)
	password := issuedPassword.FindStringSubmatch(text)
	require.NotNil(s.t, login, "no login in %q", text)
	require.NotNil(s.t, password, "no hidden password in %q", text)
	return navidrome.Account{Login: login[1], Password: password[1]}
}

func (s *Scenario) Link(from User, account navidrome.Account) *models.Update {
	s.t.Helper()

	s.Open(from, s.Catalog(from).AccountsButton())
	link := s.Catalog(from).Link()
	if !slices.Contains(telegram.ButtonTexts(s.Telegram.Buttons(s.t)), link) {
		link = s.Catalog(from).LinkAnother()
	}
	s.Go(from, link)
	return s.SendText(from, account.Login+" "+account.Password)
}

func (s *Scenario) SendText(from User, text string) *models.Update {
	msg := s.TextMessage(from, text)
	s.Send(msg)
	return msg
}

// Register uses the newcomer's username as the Navidrome login.
func (s *Scenario) Register(newcomer User) navidrome.Account {
	s.t.Helper()

	s.Send(s.TextMessage(newcomer, "/start "+s.Invite()))
	s.SendText(newcomer, newcomer.Username)
	return s.IssuedAccount()
}

// NavidromeLibrary is a library made in Navidrome by hand, like the one a
// person kept before the bot.
type NavidromeLibrary struct {
	ID int
	// Dir is where its files lie on the host; Path is where Navidrome sees them.
	Dir  string
	Path string
}

// NewNavidromeLibrary copies the audio files into a new library's root,
// outside music_dir.
func (s *Scenario) NewNavidromeLibrary(name string, files ...string) NavidromeLibrary {
	s.t.Helper()
	return s.newNavidromeLibrary(s.navidromeRoot, navidrome.LibraryMount, name, files)
}

// NewMusicDirLibrary is NewNavidromeLibrary inside music_dir, next to the
// bot's own folders.
func (s *Scenario) NewMusicDirLibrary(name string, files ...string) NavidromeLibrary {
	s.t.Helper()
	return s.newNavidromeLibrary(s.Library, s.config.NavidromeMusicDir, name, files)
}

// NewScratchLibrary is NewNavidromeLibrary inside the bot's scratch folder.
func (s *Scenario) NewScratchLibrary(name string) NavidromeLibrary {
	s.t.Helper()

	require.NoError(s.t, os.MkdirAll(s.scratchDir(), 0o755)) //nolint:gosec // G301: Navidrome container reads the library
	return s.newNavidromeLibrary(s.scratchDir(), s.NavidromePath(disk.ScratchDir), name, nil)
}

func (s *Scenario) newNavidromeLibrary(hostParent, navidromeParent, name string, files []string) NavidromeLibrary {
	s.t.Helper()

	dir, err := os.MkdirTemp(hostParent, name+"-")
	require.NoError(s.t, err)
	ndPath := navidromeParent + "/" + filepath.Base(dir)
	nd := s.Navidrome
	s.t.Cleanup(func() {
		nd.DeleteLibrariesUnder(s.t, ndPath)
		_ = os.RemoveAll(dir)
	})
	require.NoError(s.t, os.Chmod(dir, 0o755)) //nolint:gosec // G302: Navidrome container reads the library
	for _, file := range files {
		s.AddToNavidromeLibrary(NavidromeLibrary{Dir: dir}, file, filepath.Base(file))
	}
	return NavidromeLibrary{ID: s.Navidrome.CreateLibrary(s.t, filepath.Base(dir), ndPath), Dir: dir, Path: ndPath}
}

// AddToNavidromeLibrary copies the file to rel within the library.
func (s *Scenario) AddToNavidromeLibrary(lib NavidromeLibrary, file, rel string) {
	s.t.Helper()

	copyFile(s.t, file, filepath.Join(lib.Dir, rel))
}

// NewNavidromeRootLibrary makes a library of everything Navidrome sees,
// music_dir included, with the files at its top.
func (s *Scenario) NewNavidromeRootLibrary(files ...string) NavidromeLibrary {
	s.t.Helper()

	lib := NavidromeLibrary{Dir: s.navidromeRoot, Path: navidrome.LibraryMount}
	for _, file := range files {
		s.AddToNavidromeLibrary(lib, file, filepath.Base(file))
	}
	lib.ID = s.Navidrome.CreateLibrary(s.t, "whole-"+filepath.Base(s.navidromeRoot), lib.Path)
	return lib
}
