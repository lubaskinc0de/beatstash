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

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/zvuk"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

const NavidromePublicURL = "https://music.example.com"

const SecretKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" //nolint:gosec // G101: test-only key

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
	app    *app.App
	stop   func()

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
// of its own, and Navidrome logins and libraries are unique.
func prepare(t *testing.T, opts ...Option) *Scenario {
	t.Helper()
	t.Parallel()

	api := telegram.New(t)
	zvukAPI := zvuk.New(t)

	library, err := os.MkdirTemp(env.libraryRoot, "scenario-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(library, 0o755)) //nolint:gosec // G302: Navidrome container reads the library

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
		IngestPollInterval:   10 * time.Millisecond,
		TelegramPollInterval: 10 * time.Millisecond,
		ZvukURL:              zvukAPI.URL(),
		SyncInterval:         time.Hour,
		MirrorRetryInterval:  50 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Scenario{t: t, Clock: clk, config: cfg, Telegram: api, Zvuk: zvukAPI, Navidrome: env.navidrome, Library: library}
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
