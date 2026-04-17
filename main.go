package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"TG-delivery/internal/app"
	"TG-delivery/internal/config"
	"TG-delivery/internal/observability"
	"TG-delivery/internal/worker"
)

func main() {
	cfg := config.Load()
	logger := observability.NewLogger(cfg.ServiceName, cfg.Environment, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch cfg.RuntimeRole {
	case "worker":
		w, err := worker.New(ctx, cfg, logger)
		if err != nil {
			logger.Error("failed to initialize worker", "error", err)
			os.Exit(1)
		}
		if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("worker exited with error", "error", err)
			os.Exit(1)
		}
	default:
		a, err := app.New(ctx, cfg, logger)
		if err != nil {
			logger.Error("failed to initialize api app", "error", err)
			os.Exit(1)
		}
		if err := a.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("api server exited with error", "error", err)
			os.Exit(1)
		}
	}

	logger.Info(fmt.Sprintf("shutdown completed (%s)", cfg.RuntimeRole))
}
