package harness

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/lubaskinc0de/beatstash/e2e/harness/navidrome"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/e2e/harness/zvuk"
	"github.com/lubaskinc0de/beatstash/internal/domain/access"
	"github.com/lubaskinc0de/beatstash/internal/infra/stall"
	tgbot "github.com/lubaskinc0de/beatstash/internal/infra/telegram/bot"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/i18n"
	app "github.com/lubaskinc0de/beatstash/internal/main"
)

// LeaseTTL outlasts a Bot API call of the double even under the load of the
// whole suite, and keeps short the wait for a crashed instance.
const LeaseTTL = 200 * time.Millisecond

// ListenLinkTTL is the config's default.
const ListenLinkTTL = 720 * time.Hour

const (
	NavidromePublicURL = "https://music.example.com"
	botMusicDir        = "bot"
	SecretKey          = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" //nolint:gosec // G101: test-only key
)

// Clock is the bot's time: it stands still until Advance.
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*clockTimer
	// touched is when a timer was last started, reset or stopped.
	touched time.Time
	// armed close once all running timers are armed at the current time.
	armed []chan struct{}
}

type clockTimer struct {
	clock    *Clock
	f        func()
	deadline time.Time
	armedAt  time.Time
	running  bool
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance fires the timers that are due by the new time.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []func()
	for _, t := range c.timers {
		if t.running && !t.deadline.After(c.now) {
			t.running = false
			due = append(due, t.f)
		}
	}
	c.mu.Unlock()

	for _, f := range due {
		f()
	}
}

func (c *Clock) AfterFunc(d time.Duration, f func()) stall.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &clockTimer{clock: c, f: f}
	c.timers = append(c.timers, t)
	t.armLocked(d)
	return t
}

// Armed closes once a timer was started, reset or stopped at the current
// time, and every timer still running was started or reset then: whoever
// holds them has acted since the last Advance. A download that ends stops
// its timer, and leaves none running.
func (c *Clock) Armed() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	armed := make(chan struct{})
	c.armed = append(c.armed, armed)
	c.notifyLocked()
	return armed
}

func (c *Clock) notifyLocked() {
	if !c.touched.Equal(c.now) {
		return
	}
	for _, t := range c.timers {
		if t.running && !t.armedAt.Equal(c.now) {
			return
		}
	}
	for _, armed := range c.armed {
		close(armed)
	}
	c.armed = nil
}

func (t *clockTimer) Reset(d time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	was := t.running
	t.armLocked(d)
	return was
}

func (t *clockTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	was := t.running
	t.running = false
	if was {
		t.clock.touched = t.clock.now
	}
	t.clock.notifyLocked()
	return was
}

func (t *clockTimer) armLocked(d time.Duration) {
	c := t.clock
	t.deadline, t.armedAt, t.running = c.now.Add(d), c.now, true
	c.touched = c.now
	c.notifyLocked()
}

type Scenario struct {
	Clock     *Clock
	Telegram  *telegram.API
	Zvuk      *zvuk.API
	Navidrome *navidrome.Server
	Library   string
	texts     *catalogStore

	t      *testing.T
	config app.Config
	// navidromeRoot is the host directory Navidrome sees as its LibraryMount.
	navidromeRoot string
	// app gets the updates the scenario sends; ctx ends when it stops.
	app  *app.App
	ctx  context.Context
	stop func()
	// running are all the instances over the scenario's database.
	running *instances

	updates *atomic.Int64
	uploads *atomic.Int64

	sharedZvukAudio zvuk.Audio
}

type instances struct {
	mu   sync.Mutex
	apps []*app.App
}

type catalogStore struct {
	once   sync.Once
	bundle *i18n.Bundle
	err    error
}

func (i *instances) add(a *app.App) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.apps = append(i.apps, a)
}

func (i *instances) remove(a *app.App) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.apps = slices.DeleteFunc(i.apps, func(other *app.App) bool { return other == a })
}

func (i *instances) all() []*app.App {
	i.mu.Lock()
	defer i.mu.Unlock()
	return slices.Clone(i.apps)
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

// prepare gives each parallel scenario isolated services and disables
// Attached Libraries, which would otherwise include other scenarios' data.
func prepare(t *testing.T, opts ...Option) *Scenario {
	t.Helper()
	t.Parallel()

	api := telegram.New(t)
	zvukAPI := zvuk.New(t)

	library := sharedNavidromeDir(t, "scenario-")

	clk := &Clock{now: time.Now()}
	cfg := app.Config{
		Clock:                  clk.Now,
		AfterFunc:              clk.AfterFunc,
		InviteTTL:              7 * 24 * time.Hour,
		Token:                  telegram.Token,
		BotAPIURL:              api.URL(),
		MaxPostSize:            50 << 20,
		DBDSN:                  NewDatabase(t),
		MusicDir:               library,
		NavidromeMusicDir:      navidrome.LibraryMount + "/" + filepath.Base(library),
		Admins:                 []access.Identity{tgbot.Identity(Admin.ID)},
		ServiceName:            "beatstash",
		DefaultLanguage:        "en",
		NavidromePublicURL:     NavidromePublicURL,
		ListenLinkTTL:          ListenLinkTTL,
		ListenLinkDownloadable: true,
		NavidromeAccessTTL:     30 * time.Second,
		SecretKey:              SecretKey,
		NavidromeUser:          navidrome.AdminUser,
		NavidromePassword:      navidrome.AdminPassword,
		NavidromeURL:           env.navidrome.URL,
		IngestWorkers:          2,
		ZvukWorkers:            2,
		ZvukPerUser:            1,
		IngestRetryDelays:      []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond},
		IngestPollInterval:     50 * time.Millisecond,
		ScratchTTL:             time.Hour,
		StallTimeout:           time.Minute,
		ReconcileInterval:      time.Hour,
		TelegramPollInterval:   200 * time.Millisecond,
		TelegramLeaseTTL:       LeaseTTL,
		FillStorageChat:        true,
		ZvukURL:                zvukAPI.URL(),
		SyncInterval:           time.Hour,
		MirrorRetryInterval:    250 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Scenario{
		t: t, Clock: clk, config: cfg, Telegram: api, Zvuk: zvukAPI,
		Navidrome: env.navidrome, Library: library, navidromeRoot: env.libraryRoot,
		running: &instances{}, updates: &atomic.Int64{}, uploads: &atomic.Int64{}, texts: &catalogStore{},
	}
}

// Catalog uses the language code in the test user and the bot's translation
// options. After a language switch, pass a User with the selected code.
func (s *Scenario) Catalog(user User) i18n.Catalog {
	s.t.Helper()

	s.texts.once.Do(func() {
		s.texts.bundle, s.texts.err = i18n.Load(i18n.Options{
			Dir:     s.config.TranslationsDir,
			Default: i18n.Language(s.config.DefaultLanguage),
			Brand:   i18n.Brand{Service: s.config.ServiceName, NavidromeURL: s.config.NavidromePublicURL},
		})
	})
	require.NoError(s.t, s.texts.err)
	return s.texts.bundle.For(s.texts.bundle.Match(user.LanguageCode))
}

func (s *Scenario) start() {
	s.t.Helper()

	a, err := app.New(s.t.Context(), s.config, bot.WithNotAsyncHandlers())
	require.NoError(s.t, err)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := a.StartWorkers(ctx)
	var once sync.Once
	s.app, s.ctx = a, ctx
	s.running.add(a)
	s.stop = func() {
		once.Do(func() {
			s.running.remove(a)
			// The database goes first, as in a crash: the instance frees
			// nothing it holds.
			_ = a.Close()
			cancel()
			<-stopped
		})
	}
	s.t.Cleanup(s.stop)
}

// StartReplica starts one more instance of the bot over the same database,
// Library and Bot API; the returned scenario sends updates to it.
func (s *Scenario) StartReplica() *Scenario {
	s.t.Helper()

	replica := *s
	replica.start()
	return &replica
}

// Stop stops the instance, even amid an update: it is gone as if it
// crashed.
func (s *Scenario) Stop() {
	s.stop()
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
	s.texts = &catalogStore{}
	s.start()
}

// WaitReconcile waits until the instance has reconciled the Libraries once
// more.
func (s *Scenario) WaitReconcile() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.WaitReconcile(ctx))
}

// WaitAttach waits until the instance has taken the Attached Libraries from
// Navidrome once more.
func (s *Scenario) WaitAttach() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.WaitAttach(ctx))
}

// ResolveSongs has the instance learn the Navidrome songs of its Tracks now;
// scenarios find them only on demand.
func (s *Scenario) ResolveSongs() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.ResolveSongs(ctx))
}

// Sync moves the Clock a Sync interval on, so every account is due, and
// runs Sync.
func (s *Scenario) Sync() {
	s.t.Helper()

	s.Clock.Advance(s.config.SyncInterval)
	s.RunSync()
}

// RunSync runs Sync and Mirror on the instance and waits for them; an
// account that is not due is not synced.
func (s *Scenario) RunSync() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.Sync(ctx))
}

// Poll waits for a whole round of the instance's poller.
func (s *Scenario) Poll() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.Poll(ctx))
}

// PollAny waits until one of the instances finishes a whole round of its
// poller. An instance stuck on a held Bot API call cannot, so the others
// have had their turn by then.
func (s *Scenario) PollAny() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	apps := s.running.all()
	polled := make(chan error, len(apps))
	for _, a := range apps {
		go func() { polled <- a.Poll(ctx) }()
	}
	require.NoError(s.t, <-polled)
}

func (s *Scenario) Send(update *models.Update) {
	s.app.Bot().ProcessUpdate(s.ctx, update)
}

// WaitIngest waits for every running instance.
func (s *Scenario) WaitIngest() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	for _, a := range s.running.all() {
		require.NoError(s.t, a.WaitIngest(ctx))
	}
}

// NewOwnNavidrome gives the scenario a separate Navidrome for tests involving
// Attached Libraries, account permissions, or library deletion. setup runs
// before the bot starts.
func NewOwnNavidrome(t *testing.T, setup func(s *Scenario), opts ...Option) *Scenario {
	t.Helper()
	return newOwnNavidrome(t, true, setup, opts...)
}

func NewOwnNavidromeWithoutSharing(t *testing.T, setup func(s *Scenario), opts ...Option) *Scenario {
	t.Helper()
	return newOwnNavidrome(t, false, setup, opts...)
}

func newOwnNavidrome(t *testing.T, sharing bool, setup func(s *Scenario), opts ...Option) *Scenario {
	t.Helper()

	s := prepare(t, append([]Option{WithAttachInterval(time.Hour)}, opts...)...)
	s.ownNavidrome(sharing)
	if setup != nil {
		setup(s)
	}
	s.start()
	s.join(Alice)
	s.join(Bob)
	s.Telegram.Forget()
	return s
}

func (s *Scenario) ownNavidrome(sharing bool) {
	s.t.Helper()

	nd, root := startNavidrome(s.t, sharing)
	s.Navidrome = nd
	s.navidromeRoot = root
	s.Library = filepath.Join(root, botMusicDir)
	s.config.MusicDir = s.Library
	s.config.NavidromeMusicDir = navidrome.LibraryMount + "/" + botMusicDir
	s.config.NavidromeURL = nd.URL
}

// startNavidrome starts a Navidrome of the test's own; its root on the host
// holds music_dir at botMusicDir.
func startNavidrome(t *testing.T, sharing bool) (server *navidrome.Server, root string) {
	t.Helper()

	root, err := os.MkdirTemp("", "beatstash-own-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	require.NoError(t, os.Chmod(root, 0o755)) //nolint:gosec // G302: Navidrome container reads the library
	for _, dir := range []string{navidrome.RootLibrary, botMusicDir} {
		require.NoError(t, os.Mkdir(filepath.Join(root, dir), 0o755)) //nolint:gosec // G301: Navidrome container reads the library
	}

	server, container, err := navidrome.Start(t.Context(), root, sharing)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container, testcontainers.StopTimeout(0)) })
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
