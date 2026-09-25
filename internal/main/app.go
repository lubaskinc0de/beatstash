package app

import (
	"context"
	"sync"

	"github.com/go-telegram/bot"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/secrets"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
)

type App struct {
	bot     *bot.Bot
	workers *ingest.Workers
	db      *gorm.DB

	closeOnce sync.Once
	closeErr  error
}

// New wires the whole application into a bot ready to process updates and
// brings the database, music_dir and Navidrome in step on the way.
// Extra options are applied after the configured ones.
func New(ctx context.Context, cfg Config, opts ...bot.Option) (*App, error) {
	db, err := database.New(cfg.DBDSN)
	if err != nil {
		return nil, err
	}

	a, err := build(ctx, cfg, db, opts)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	return a, nil
}

func build(ctx context.Context, cfg Config, db *gorm.DB, opts []bot.Option) (*App, error) {
	if err := database.Migrate(db); err != nil {
		return nil, err
	}
	box, err := secrets.NewBox(cfg.SecretKey)
	if err != nil {
		return nil, err
	}

	txManager := &database.TxManager{DB: db}
	tracks := &database.TrackRepository{DB: db}
	users := &database.UserRepository{DB: db}
	uploads := &database.UploadRepository{DB: db}
	libraryRepo := &database.LibraryRepository{DB: db}
	libraryLock := &database.LibraryLock{DB: db}
	ingestQueue := &database.IngestQueue{DB: db}
	accountRepo := &database.NavidromeAccountRepository{DB: db}
	navidromeClient := navidrome.NewClient(cfg.NavidromeURL, &database.NavidromeSessionRepository{DB: db})
	navidromeAdmin := application.NavidromeCredentials{Login: cfg.NavidromeUser, Password: cfg.NavidromePassword}
	accounts := &application.NavidromeAccounts{Repo: accountRepo, Box: box}

	libraries := &application.Libraries{
		Repo:              libraryRepo,
		Users:             users,
		Accounts:          accountRepo,
		Navidrome:         navidromeClient,
		Admin:             navidromeAdmin,
		MusicDir:          cfg.MusicDir,
		NavidromeMusicDir: cfg.NavidromeMusicDir,
	}
	startup := &application.Startup{Admins: users, AdminIDs: cfg.AdminIDs, Libraries: libraries}
	if err := startup.Execute(ctx); err != nil {
		return nil, err
	}

	options := tgbot.Options(&application.Authenticate{Users: users})
	if cfg.BotAPIURL != "" {
		options = append(options, bot.WithServerURL(cfg.BotAPIURL))
	}
	b, err := bot.New(cfg.Token, append(options, opts...)...)
	if err != nil {
		return nil, err
	}

	sharing := &library.Sharing{
		Tx:        txManager,
		Tracks:    tracks,
		Shares:    &database.ShareRepository{DB: db},
		Takes:     &database.TakeRepository{DB: db},
		Libraries: libraries,
		Lock:      libraryLock,
		Clock:     cfg.Clock,
	}
	pipeline := &ingest.Pipeline{
		Providers: application.NewProviders(&tgprovider.Provider{Bot: b}),
		Tracks:    tracks,
		Uploads:   uploads,
		Libraries: libraryRepo,
		Shared:    sharing,
		Lock:      libraryLock,
		MusicDir:  cfg.MusicDir,
	}
	workers := ingest.NewWorkers(
		txManager,
		ingestQueue,
		pipeline,
		&tgbot.Notifier{Bot: b},
		cfg.IngestWorkers,
		cfg.IngestRetryDelays,
		cfg.IngestPollInterval,
	)
	invites := &database.InviteRepository{DB: db}
	createInvite := &application.CreateInvite{Invites: invites, AdminIDs: cfg.AdminIDs, TTL: cfg.InviteTTL, Clock: cfg.Clock}

	handler := &tgbot.Handler{
		EnqueueIngest: &application.EnqueueIngest{Queue: ingestQueue, Waker: workers},
		GetNowPlaying: &application.GetNowPlaying{
			Client:    navidromeClient,
			Repo:      tracks,
			Accounts:  accounts,
			Libraries: libraries,
		},
		GetRecentlyPlayed: &application.GetRecentlyPlayed{
			Client:    navidromeClient,
			Repo:      tracks,
			Accounts:  accounts,
			Libraries: libraries,
		},
		LinkNavidromeAccount: &application.LinkNavidromeAccount{
			Navidrome: navidromeClient,
			Accounts:  accounts,
			Users:     users,
			Libraries: libraries,
		},
		CreateInvite: createInvite,
		AcceptInvite: &application.AcceptInvite{
			Tx:        txManager,
			Invites:   invites,
			Users:     users,
			Libraries: libraries,
			Clock:     cfg.Clock,
		},
		RegisterAccount: &application.RegisterNavidromeAccount{
			Navidrome: navidromeClient,
			Accounts:  accounts,
			Users:     users,
			Libraries: libraries,
			Admin:     navidromeAdmin,
		},
		Sharing:         sharing,
		GetTop:          &application.GetTop{Shares: sharing.Shares, Takes: sharing.Takes, Clock: cfg.Clock},
		GetServiceStats: &application.GetServiceStats{Users: users, Tracks: tracks, Libraries: libraryRepo},
		AdminContact:    cfg.AdminContact,
	}
	handler.Register(b)

	return &App{bot: b, workers: workers, db: db}, nil
}

func (a *App) Bot() *bot.Bot {
	return a.bot
}

// Run serves updates and Ingest Jobs until ctx is done.
func (a *App) Run(ctx context.Context) {
	stopped := a.StartWorkers(ctx)
	a.bot.Start(ctx)
	<-stopped
}

func (a *App) StartWorkers(ctx context.Context) <-chan struct{} {
	return a.workers.Start(ctx)
}

func (a *App) WaitIngest(ctx context.Context) error {
	return a.workers.WaitIdle(ctx)
}

func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.closeErr = database.Close(a.db)
	})
	return a.closeErr
}
