package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"TG-delivery/internal/config"
	"TG-delivery/internal/observability"
	"TG-delivery/internal/transport/httpapi/handlers"
	adminhandlers "TG-delivery/internal/transport/httpapi/handlers/admin"
	publichandlers "TG-delivery/internal/transport/httpapi/handlers/public"
	webhookhandlers "TG-delivery/internal/transport/httpapi/handlers/webhooks"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Dependencies struct {
	AvailabilityHandler *adminhandlers.AvailabilityHandler
	AdminMenuHandler    *adminhandlers.MenuHandler
	OrdersHandler       *adminhandlers.OrdersHandler
	AdminPayments       *adminhandlers.PaymentsHandler
	AdminRefunds        *adminhandlers.RefundsHandler
	MenuHandler         *publichandlers.MenuHandler
	CartHandler         *publichandlers.CartHandler
	CheckoutHandler     *publichandlers.CheckoutHandler
	PaymentsHandler     *publichandlers.PaymentsHandler
	OrdersPublicHandler *publichandlers.OrdersHandler
	AddressesHandler    *publichandlers.AddressesHandler
	DeliveryHandler     *publichandlers.DeliveryHandler
	MockPaymentWebhook  *webhookhandlers.MockPaymentHandler
	TelegramWebhook     *webhookhandlers.TelegramHandler
}

func NewRouter(cfg config.Config, logger *slog.Logger, checkDB func() error, deps Dependencies, metrics *observability.Metrics) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(middleware.CORS)
	r.Use(middleware.RequestID)
	r.Use(middleware.HTTPMetrics(metrics))
	r.Use(middleware.AccessLog(logger))

	health := handlers.HealthHandler{
		ServiceName: cfg.ServiceName,
		CheckDB:     checkDB,
	}

	r.Get("/health/live", health.Liveness)
	r.Get("/health/ready", health.Readiness)
	r.Get("/health", health.Readiness)
	if metrics != nil {
		r.Handle("/metrics", metrics)
	}

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("pong"))
		})

		if deps.MenuHandler != nil {
			api.Get("/menu/branches/{branchID}", deps.MenuHandler.ListBranchMenu)
		}
		if deps.CartHandler != nil {
			api.Get("/cart", deps.CartHandler.GetActiveCart)
			api.Post("/cart/items", deps.CartHandler.UpsertCartItem)
			api.Delete("/cart/items/{cartItemID}", deps.CartHandler.DeleteCartItem)
		}
		if deps.CheckoutHandler != nil {
			api.Post("/checkout/draft", deps.CheckoutHandler.CreateDraft)
		}
		if deps.PaymentsHandler != nil {
			api.Post("/payments/sessions", deps.PaymentsHandler.CreateSession)
		}
		if deps.OrdersPublicHandler != nil {
			api.Get("/orders", deps.OrdersPublicHandler.ListUserOrders)
			api.Post("/orders/{orderID}/repeat", deps.OrdersPublicHandler.RepeatOrder)
		}
		if deps.AddressesHandler != nil {
			api.Get("/addresses", deps.AddressesHandler.List)
			api.Post("/addresses", deps.AddressesHandler.Upsert)
			api.Delete("/addresses/{addressID}", deps.AddressesHandler.Delete)
		}
		if deps.DeliveryHandler != nil {
			api.Post("/delivery/quote", deps.DeliveryHandler.Quote)
		}

		if deps.AvailabilityHandler != nil || deps.AdminMenuHandler != nil || deps.OrdersHandler != nil || deps.AdminPayments != nil || deps.AdminRefunds != nil {
			api.Route("/admin", func(admin chi.Router) {
				admin.Use(middleware.RequireAdminToken(cfg.Security.AdminToken))
				if deps.AdminMenuHandler != nil {
					admin.Get("/branches/{branchID}/menu", deps.AdminMenuHandler.ListBranchMenu)
					admin.Put("/branches/{branchID}/menu-items/{menuItemID}", deps.AdminMenuHandler.UpdateBranchMenuItem)
				}
				if deps.AvailabilityHandler != nil {
					admin.Get("/branches/{branchID}/stop-list", deps.AvailabilityHandler.ListStopList)
					admin.Put("/branches/{branchID}/menu-items/{menuItemID}/availability", deps.AvailabilityHandler.UpdateAvailability)
				}
				if deps.OrdersHandler != nil {
					admin.Get("/orders/manual-review", deps.OrdersHandler.ListManualReview)
					admin.Post("/orders/{orderID}/manual-review/resolve", deps.OrdersHandler.ResolveManualReview)
				}
				if deps.AdminPayments != nil {
					admin.Get("/payments", deps.AdminPayments.List)
				}
				if deps.AdminRefunds != nil {
					admin.Get("/refunds", deps.AdminRefunds.List)
					admin.Post("/orders/{orderID}/refunds", deps.AdminRefunds.Request)
				}
			})
		}

		if deps.MockPaymentWebhook != nil {
			api.Post("/webhooks/payments/mock", deps.MockPaymentWebhook.Ingest)
		}
		if deps.TelegramWebhook != nil {
			api.Post("/webhooks/telegram", deps.TelegramWebhook.Ingest)
		}
	})

	return r
}
