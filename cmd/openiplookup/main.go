package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/app"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	logger := newLogger(os.Getenv("APP_ENV"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load("config.json")
	if err != nil {
		logger.Error("load config", slog.Any("err", err))
		return err
	}
	logger.Info("config loaded successfully")

	mgr := app.New(cfg, logger)
	if err := mgr.Run(ctx); err != nil {
		logger.Error("run app", slog.Any("err", err))
		return err
	}
	return nil
}

func newLogger(env string) *slog.Logger {
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
