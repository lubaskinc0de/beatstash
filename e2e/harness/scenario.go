package harness

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

const (
	NavidromePublicURL = "https://music.example.com"
	botMusicDir        = "bot"
	SecretKey          = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" //nolint:gosec // G101: test-only key
)

type Clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type Scenario struct {
	Clock     *Clock
	Telegram  *telegram.API
	Zvuk      *zvuk.API
	Navidrome *navidrome.Server
	Library   string

	t      *testing.T
	config app.Config
	// navidromeRoot is the host directory Navidrome sees as its LibraryMount.
	navidromeRoot string
	app           *app.App
	stop          func()

	updates int64
	uploads int

	sharedZvukAudio zvuk.Audio
}

type Option func(*app.Config)

func New(t *testing.T, opts ...Option) *Scenario {
	t.Helper()

	s := prepare(t, opts...)
	s.start()
	s.join(Alice)
	s.join(Bob)
	s.Telegram.Forget()
	return s
}

// prepare sets up everything a bot needs but does not start it.
// Scenarios run in parallel: each has a database, Bot API, Zvuk and Library
// of its own, and Navidrome logins and libraries are unique. Attached
// Libraries are off: the bot would attach the libraries of other scenarios.
func prepare(t *testing.T, opts ...Option) *Scenario {
	t.Helper()
	t.Parallel()

	api := telegram.New(t)
	zvukAPI := zvuk.New(t)

	library := sharedNavidromeDir(t, "scenario-")

	clk := &Clock{now: time.Now()}
	cfg := app.Config{
		Clock:                clk.Now,
		InviteTTL:            7 * 24 * time.Hour,
		Token:                telegram.Token,
		BotAPIURL:            api.URL(),
		MaxPostSize:          50 << 20,
		DBDSN:                NewDatabase(t),
		MusicDir:             library,
		NavidromeMusicDir:    navidrome.LibraryMount + "/" + filepath.Base(library),
		Admins:               []access.Identity{tgbot.Identity(Admin.ID)},
		ServiceName:          "navidrome-tg",
		DefaultLanguage:      "en",
		NavidromePublicURL:   NavidromePublicURL,
		SecretKey:            SecretKey,
		NavidromeUser:        navidrome.AdminUser,
		NavidromePassword:    navidrome.AdminPassword,
		NavidromeURL:         env.navidrome.URL,
		IngestWorkers:        2,
		ZvukWorkers:          2,
		ZvukPerUser:          1,
		IngestRetryDelays:    []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond},
		IngestPollInterval:   50 * time.Millisecond,
		TelegramPollInterval: 200 * time.Millisecond,
		ZvukURL:              zvukAPI.URL(),
		SyncInterval:         time.Hour,
		MirrorRetryInterval:  250 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Scenario{
		t: t, Clock: clk, config: cfg, Telegram: api, Zvuk: zvukAPI,
		Navidrome: env.navidrome, Library: library, navidromeRoot: env.libraryRoot,
	}
}

func (s *Scenario) start() {
	s.t.Helper()

	a, err := app.New(s.t.Context(), s.config, bot.WithSkipGetMe(), bot.WithNotAsyncHandlers())
	require.NoError(s.t, err)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := a.StartWorkers(ctx)
	var once sync.Once
	s.app = a
	s.stop = func() {
		once.Do(func() {
			cancel()
			<-stopped
			_ = a.Close()
		})
	}
	s.t.Cleanup(s.stop)
}

// Restart stops the bot, even mid-Ingest, and starts it again on the same
// database and Library, this time with Ingest workers.
func (s *Scenario) Restart(opts ...Option) {
	s.t.Helper()

	s.stop()
	s.config.IngestWorkers = 2
	s.config.ZvukWorkers = 2
	for _, opt := range opts {
		opt(&s.config)
	}
	s.start()
}

func (s *Scenario) Send(update *models.Update) {
	s.app.Bot().ProcessUpdate(s.t.Context(), update)
}

func (s *Scenario) WaitIngest() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.WaitIngest(ctx))
}

// NewOwnNavidrome runs the bot on a Navidrome of its own, with Attached
// Libraries on, for scenarios the shared Navidrome would mix up: Attached
// Libraries, accounts the bot creates, deleted libraries. setup, if any,
// fills Navidrome before the bot starts.
func NewOwnNavidrome(t *testing.T, setup func(s *Scenario), opts ...Option) *Scenario {
	t.Helper()

	s := prepare(t, append([]Option{WithAttachInterval(time.Hour)}, opts...)...)
	s.ownNavidrome()
	if setup != nil {
		setup(s)
	}
	s.start()
	s.join(Alice)
	s.join(Bob)
	s.Telegram.Forget()
	return s
}

func (s *Scenario) ownNavidrome() {
	s.t.Helper()

	nd, root := StartNavidrome(s.t)
	s.Navidrome = nd
	s.navidromeRoot = root
	s.Library = filepath.Join(root, botMusicDir)
	s.config.MusicDir = s.Library
	s.config.NavidromeMusicDir = navidrome.LibraryMount + "/" + botMusicDir
	s.config.NavidromeURL = nd.URL
}

// StartNavidrome starts a Navidrome of the test's own; its root on the host
// holds music_dir at botMusicDir.
func StartNavidrome(t *testing.T) (server *navidrome.Server, root string) {
	t.Helper()

	root, err := os.MkdirTemp("", "navidrome-tg-own-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	require.NoError(t, os.Chmod(root, 0o755)) //nolint:gosec // G302: Navidrome container reads the library
	for _, dir := range []string{navidrome.RootLibrary, botMusicDir} {
		require.NoError(t, os.Mkdir(filepath.Join(root, dir), 0o755)) //nolint:gosec // G301: Navidrome container reads the library
	}

	server, container, err := navidrome.Start(t.Context(), root)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	require.NoError(t, err)
	return server, root
}

// sharedNavidromeDir makes a directory the shared Navidrome sees. It goes
// with its libraries when the scenario ends: Navidrome scans them all each
// time a library is created.
func sharedNavidromeDir(t *testing.T, prefix string) string {
	t.Helper()

	dir, err := os.MkdirTemp(env.libraryRoot, prefix)
	require.NoError(t, err)
	t.Cleanup(func() {
		env.navidrome.DeleteLibrariesUnder(t, navidrome.LibraryMount+"/"+filepath.Base(dir))
		_ = os.RemoveAll(dir)
	})
	require.NoError(t, os.Chmod(dir, 0o755)) //nolint:gosec // G302: Navidrome container reads the library
	return dir
}
