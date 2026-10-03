package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	app "github.com/lubaskinc0de/beatstash/internal/main"
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
	os.Exit(run())
}

func run() int {
	cfg, err := app.LoadConfig()
	if err != nil {
		slog.Error("config_invalid", "error", err)
		return 1
	}
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	b, err := app.New(ctx, cfg)
	if err != nil {
		slog.Error("app_initialization_failed", "error", err)
		return 1
	}

	slog.Info("bot_started")
	b.Run(ctx)
	_ = b.Close()
	return 0
}
