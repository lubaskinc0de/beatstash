package app

import (
	"context"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

// New wires the whole application into a bot ready to process updates.
// Extra options are applied after the configured ones.
func New(cfg Config, opts ...bot.Option) (*App, error) {
	db, err := database.New(cfg.DbDsn)
	if err != nil {
		return nil, err
	}

	a, err := build(cfg, db, opts)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	return a, nil
}

func build(cfg Config, db *gorm.DB, opts []bot.Option) (*App, error) {
	if err := database.Migrate(db); err != nil {
		return nil, err
	}

	txManager := database.NewTxManager(db)
	trackRepo := database.NewTrackRepository(db)
	userRepo := database.NewUserRepository(db)
	inviteRepo := database.NewInviteRepository(db)
	sessionRepo := database.NewNavidromeSessionRepository(db)
	uploadRepo := database.NewUploadRepository(db)
	libraryLock := database.NewLibraryLock(db)
	ingestQueue := database.NewIngestQueue(db)
	navidromeClient := navidrome.NewClient(cfg.NavidromeUrl, sessionRepo)
	box, err := secrets.NewBox(cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	accountRepo := database.NewNavidromeAccountRepository(db)
	navidromeAccounts := application.NewNavidromeAccounts(accountRepo, box)
	libraryRepo := database.NewLibraryRepository(db)
	navidromeAdmin := application.NavidromeCredentials{Login: cfg.NavidromeUser, Password: cfg.NavidromePassword}
	libraries := &application.Libraries{
		Repo:              libraryRepo,
		Users:             userRepo,
		Accounts:          accountRepo,
		Navidrome:         navidromeClient,
		Admin:             navidromeAdmin,
		MusicDir:          cfg.MusicDir,
		NavidromeMusicDir: cfg.NavidromeMusicDir,
	}

	if err := userRepo.EnsureExist(context.Background(), cfg.AdminIds); err != nil {
		return nil, err
	}
	if err := libraries.Prepare(context.Background()); err != nil {
		return nil, err
	}

	options := []bot.Option{
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			"message",
			"inline_query",
			"callback_query",
		}),
		bot.WithMiddlewares(
			tgbot.UserMiddleware(userRepo),
		),
	}
	if cfg.BotApiUrl != "" {
		options = append(options, bot.WithServerURL(cfg.BotApiUrl))
	}
	options = append(options, opts...)

	b, err := bot.New(cfg.Token, options...)
	if err != nil {
		return nil, err
	}

	sharing := &library.Sharing{
		Tx:        txManager,
		Tracks:    trackRepo,
		Shares:    database.NewShareRepository(db),
		Takes:     database.NewTakeRepository(db),
		Libraries: libraries,
		Lock:      libraryLock,
		Clock:     cfg.Clock,
	}
	providers := application.NewProviders(tgprovider.NewProvider(b))
	workers := ingest.NewWorkers(
		txManager,
		ingestQueue,
		ingest.NewPipeline(providers, trackRepo, uploadRepo, libraryRepo, sharing, libraryLock, cfg.MusicDir),
		tgbot.NewNotifier(b),
		cfg.IngestWorkers,
		cfg.IngestRetryDelays,
		cfg.IngestPollInterval,
	)
	enqueueIngest := application.NewEnqueueIngest(ingestQueue, workers)
	nowPlaying := application.NewGetNowPlaying(navidromeClient, trackRepo, navidromeAccounts, libraries)
	recentlyPlayed := application.NewGetRecentlyPlayed(navidromeClient, trackRepo, navidromeAccounts, libraries)

	handler := tgbot.NewHandler(
		enqueueIngest,
		nowPlaying,
		recentlyPlayed,
		application.NewLinkNavidromeAccount(navidromeClient, navidromeAccounts, userRepo, libraries),
		application.NewCreateInvite(inviteRepo, cfg.AdminIds, cfg.InviteTTL, cfg.Clock),
		application.NewAcceptInvite(txManager, inviteRepo, userRepo, libraries, cfg.Clock),
		application.NewRegisterNavidromeAccount(
			navidromeClient,
			navidromeAccounts,
			userRepo,
			libraries,
			navidromeAdmin,
		),
		sharing,
		application.NewGetTop(sharing.Shares, sharing.Takes, cfg.Clock),
		&application.GetServiceStats{Users: userRepo, Tracks: trackRepo, Libraries: libraryRepo},
		cfg.AdminContact,
	)

	b.RegisterHandlerMatchFunc(tgbot.HasAudio, handler.HandleAudio)
	b.RegisterHandlerMatchFunc(tgbot.IsCommand("link"), handler.HandleLink)
	b.RegisterHandlerMatchFunc(tgbot.IsCommand("invite"), handler.HandleInvite)
	b.RegisterHandlerMatchFunc(tgbot.IsCommand("start"), handler.HandleStart)
	b.RegisterHandlerMatchFunc(tgbot.IsCommand("share"), handler.HandleShare)
	b.RegisterHandlerMatchFunc(tgbot.IsCommand("shared"), handler.HandleSharedFeed)
	b.RegisterHandlerMatchFunc(tgbot.IsCommand("top"), handler.HandleTop)
	b.RegisterHandlerMatchFunc(tgbot.IsText, handler.HandleText)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.InlineQuery != nil
		},
		handler.HandleInlineQuery,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.CallbackQuery != nil
		},
		handler.HandleCallbackQuery,
	)

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
