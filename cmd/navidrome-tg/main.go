package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	app "github.com/lubaskinc0de/navidrome-tg/internal/main"
)

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
	cfg := app.LoadConfig()
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	b, err := app.New(cfg)
	if err != nil {
		slog.Error("app_initialization_failed", "error", err)
		os.Exit(1)
	}

	slog.Info("bot_started")
	b.Run(ctx)
	_ = b.Close()
}
