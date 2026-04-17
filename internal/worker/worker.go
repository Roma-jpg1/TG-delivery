package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"TG-delivery/internal/config"
	"TG-delivery/internal/storage/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	cfg    config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Worker, error) {
	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	return &Worker{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.db.Close()

	w.logger.Info("worker started", "poll_interval", w.cfg.Worker.OutboxPollInterval.String(), "batch_size", w.cfg.Worker.BatchSize)
	ticker := time.NewTicker(w.cfg.Worker.OutboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return nil
		case <-ticker.C:
			// Placeholder for outbox polling + saga orchestration.
			w.logger.Debug("worker tick")
		}
	}
}
