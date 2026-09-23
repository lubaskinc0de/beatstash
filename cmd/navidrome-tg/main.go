package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
	"github.com/lubaskinc0de/navidrome-tg/internal/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/telegram"
)

type Dependencies struct {
	Handler *telegram.Handler
	users   *database.UserRepository
}

func setupDeps(config config.Config) (*Dependencies, error) {
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

	return &Dependencies{
		Handler: handler,
		users:   userRepo,
	}, nil
}

func setupLogger() {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)

	slog.SetDefault(logger)
}

func main() {
	setupLogger()
	config := config.LoadConfig()
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	deps, err := setupDeps(config)
	if err != nil {
		slog.Error("deps_initialization_failed", "error", err)
		os.Exit(1)
	}

	b, err := bot.New(
		config.Token,
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			"message",
			"inline_query",
			"callback_query",
		}),
		bot.WithMiddlewares(
			telegram.UserMiddleware(deps.users, config.AllowedUserIds),
		),
	)
	if err != nil {
		slog.Error("bot_initialization_failed", "error", err)
		os.Exit(1)
	}

	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"",
		bot.MatchTypePrefix,
		deps.Handler.HandleTrack,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.InlineQuery != nil
		},
		deps.Handler.HandleInlineQuery,
	)

	slog.Info("bot_started")
	b.Start(ctx)
}
