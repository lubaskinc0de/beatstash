package app

import (
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/telegram"
)

type dependencies struct {
	handler *telegram.Handler
	users   *database.UserRepository
}

func setupDeps(config config.Config) (*dependencies, error) {
	db, err := database.New(config.DbDsn)
	if err != nil {
		return nil, err
	}

	if err := database.Migrate(db); err != nil {
		return nil, err
	}

	// infrastructure
	txManager := database.NewTxManager(db)
	trackRepo := database.NewTrackRepository(db)
	userRepo := database.NewUserRepository(db)
	sessionRepo := database.NewNavidromeSessionRepository(db)
	navidromeClient := application.NewNavidromeClient(
		config.NavidromeUrl,
		config.NavidromePassword,
		config.NavidromeUser,
		sessionRepo,
	)

	// application
	saveTrack := application.NewSaveTrack(trackRepo, txManager)
	nowPlaying := application.NewGetNowPlaying(navidromeClient, trackRepo)
	recentlyPlayed := application.NewGetRecentlyPlayed(navidromeClient, trackRepo)

	// delivery
	handler := telegram.NewHandler(
		saveTrack,
		nowPlaying,
		recentlyPlayed,
		userRepo,
		config,
	)

	return &dependencies{
		handler: handler,
		users:   userRepo,
	}, nil
}

// New wires the whole application into a bot ready to process updates.
// Extra options are applied after the configured ones.
func New(config config.Config, opts ...bot.Option) (*bot.Bot, error) {
	deps, err := setupDeps(config)
	if err != nil {
		return nil, err
	}

	options := []bot.Option{
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			"message",
			"inline_query",
			"callback_query",
		}),
		bot.WithMiddlewares(
			telegram.UserMiddleware(deps.users, config.AllowedUserIds),
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

	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"",
		bot.MatchTypePrefix,
		deps.handler.HandleTrack,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.InlineQuery != nil
		},
		deps.handler.HandleInlineQuery,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.CallbackQuery != nil
		},
		deps.handler.HandleCallbackQuery,
	)

	return b, nil
}
