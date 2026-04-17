package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"TG-delivery/internal/config"
	"TG-delivery/internal/modules/availability"
	"TG-delivery/internal/modules/cart"
	"TG-delivery/internal/modules/checkout"
	"TG-delivery/internal/modules/menu"
	"TG-delivery/internal/modules/orders"
	"TG-delivery/internal/modules/payments"
	webhooksmodule "TG-delivery/internal/modules/webhooks"
	"TG-delivery/internal/storage/postgres"
	"TG-delivery/internal/transport/httpapi"
	adminhandlers "TG-delivery/internal/transport/httpapi/handlers/admin"
	publichandlers "TG-delivery/internal/transport/httpapi/handlers/public"
	webhookhandlers "TG-delivery/internal/transport/httpapi/handlers/webhooks"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	http   *http.Server
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	db, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	availabilityRepo := availability.NewPostgresRepository(db)
	availabilityService := availability.NewService(availabilityRepo)
	availabilityHandler := adminhandlers.NewAvailabilityHandler(availabilityService)
	menuHandler := publichandlers.NewMenuHandler(menu.NewService(db))
	cartHandler := publichandlers.NewCartHandler(cart.NewService(db))
	checkoutHandler := publichandlers.NewCheckoutHandler(checkout.NewService(db))
	paymentsHandler := publichandlers.NewPaymentsHandler(payments.NewService(db))
	ordersHandler := adminhandlers.NewOrdersHandler(orders.NewService(db))
	mockWebhookHandler := webhookhandlers.NewMockPaymentHandler(webhooksmodule.NewMockPaymentService(db, cfg.Webhooks.MockPaymentSecret))
	telegramWebhookHandler := webhookhandlers.NewTelegramHandler(webhooksmodule.NewTelegramService(db, cfg.Webhooks.TelegramSecret))

	router := httpapi.NewRouter(cfg, logger, func() error {
		return postgres.Ping(context.Background(), db, cfg.Database.HealthcheckTimeout)
	}, httpapi.Dependencies{
		AvailabilityHandler: availabilityHandler,
		OrdersHandler:       ordersHandler,
		MenuHandler:         menuHandler,
		CartHandler:         cartHandler,
		CheckoutHandler:     checkoutHandler,
		PaymentsHandler:     paymentsHandler,
		MockPaymentWebhook:  mockWebhookHandler,
		TelegramWebhook:     telegramWebhookHandler,
	})

	httpServer := &http.Server{
		Addr:         cfg.HTTP.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	return &App{
		cfg:    cfg,
		logger: logger,
		db:     db,
		http:   httpServer,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("api server started", "address", a.cfg.HTTP.Address)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return a.shutdown(context.Background())
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}
}

func (a *App) shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := a.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if a.db != nil {
		a.db.Close()
	}

	a.logger.Info("api server stopped")
	return nil
}
