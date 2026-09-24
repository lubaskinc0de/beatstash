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
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/navidrome"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/providers/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/secrets"
	"github.com/lubaskinc0de/navidrome-tg/internal/telegram"
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
func New(config config.Config, opts ...bot.Option) (*App, error) {
	db, err := database.New(config.DbDsn)
	if err != nil {
		return nil, err
	}

	a, err := build(config, db, opts)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	return a, nil
}

func build(config config.Config, db *gorm.DB, opts []bot.Option) (*App, error) {
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
	navidromeClient := navidrome.NewClient(config.NavidromeUrl, sessionRepo)
	box, err := secrets.NewBox(config.SecretKey)
	if err != nil {
		return nil, err
	}
	accountRepo := database.NewNavidromeAccountRepository(db)
	navidromeAccounts := application.NewNavidromeAccounts(accountRepo, box)
	libraryRepo := database.NewLibraryRepository(db)
	navidromeAdmin := application.NavidromeCredentials{Login: config.NavidromeUser, Password: config.NavidromePassword}
	libraries := &application.Libraries{
		Repo:              libraryRepo,
		Users:             userRepo,
		Accounts:          accountRepo,
		Navidrome:         navidromeClient,
		Admin:             navidromeAdmin,
		MusicDir:          config.MusicDir,
		NavidromeMusicDir: config.NavidromeMusicDir,
	}

	if err := userRepo.EnsureExist(context.Background(), config.AdminIds); err != nil {
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
			telegram.UserMiddleware(userRepo),
		),
	}
	if config.BotApiUrl != "" {
		options = append(options, bot.WithServerURL(config.BotApiUrl))
	}
	options = append(options, opts...)

	b, err := bot.New(config.Token, options...)
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
		Clock:     config.Clock,
	}
	providers := application.NewProviders(tgprovider.NewProvider(b))
	workers := ingest.NewWorkers(
		txManager,
		ingestQueue,
		ingest.NewPipeline(providers, trackRepo, uploadRepo, libraryRepo, sharing, libraryLock, config.MusicDir),
		telegram.NewNotifier(b),
		config.IngestWorkers,
		config.IngestRetryDelays,
		config.IngestPollInterval,
	)
	enqueueIngest := application.NewEnqueueIngest(ingestQueue, workers)
	nowPlaying := application.NewGetNowPlaying(navidromeClient, trackRepo, navidromeAccounts, libraries)
	recentlyPlayed := application.NewGetRecentlyPlayed(navidromeClient, trackRepo, navidromeAccounts, libraries)

	handler := telegram.NewHandler(
		enqueueIngest,
		nowPlaying,
		recentlyPlayed,
		application.NewLinkNavidromeAccount(navidromeClient, navidromeAccounts, userRepo, libraries),
		application.NewCreateInvite(inviteRepo, config.AdminIds, config.InviteTTL, config.Clock),
		application.NewAcceptInvite(txManager, inviteRepo, userRepo, libraries, config.Clock),
		application.NewRegisterNavidromeAccount(
			navidromeClient,
			navidromeAccounts,
			userRepo,
			libraries,
			navidromeAdmin,
		),
		sharing,
		application.NewGetTop(sharing.Shares, sharing.Takes, config.Clock),
		&application.GetServiceStats{Users: userRepo, Tracks: trackRepo, Libraries: libraryRepo},
		config.AdminContact,
	)

	b.RegisterHandlerMatchFunc(telegram.HasAudio, handler.HandleAudio)
	b.RegisterHandlerMatchFunc(telegram.IsCommand("link"), handler.HandleLink)
	b.RegisterHandlerMatchFunc(telegram.IsCommand("invite"), handler.HandleInvite)
	b.RegisterHandlerMatchFunc(telegram.IsCommand("start"), handler.HandleStart)
	b.RegisterHandlerMatchFunc(telegram.IsCommand("share"), handler.HandleShare)
	b.RegisterHandlerMatchFunc(telegram.IsCommand("shared"), handler.HandleSharedFeed)
	b.RegisterHandlerMatchFunc(telegram.IsCommand("top"), handler.HandleTop)
	b.RegisterHandlerMatchFunc(telegram.IsText, handler.HandleText)
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
