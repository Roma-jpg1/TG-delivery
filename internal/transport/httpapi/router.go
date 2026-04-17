package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"TG-delivery/internal/config"
	"TG-delivery/internal/transport/httpapi/handlers"
	adminhandlers "TG-delivery/internal/transport/httpapi/handlers/admin"
	"TG-delivery/internal/transport/httpapi/middleware"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Dependencies struct {
	AvailabilityHandler *adminhandlers.AvailabilityHandler
}

func NewRouter(cfg config.Config, logger *slog.Logger, checkDB func() error, deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(middleware.RequestID)
	r.Use(middleware.AccessLog(logger))

	health := handlers.HealthHandler{
		ServiceName: cfg.ServiceName,
		CheckDB:     checkDB,
	}

	r.Get("/health/live", health.Liveness)
	r.Get("/health/ready", health.Readiness)
	r.Get("/health", health.Readiness)

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("pong"))
		})

		if deps.AvailabilityHandler != nil {
			api.Route("/admin", func(admin chi.Router) {
				admin.Get("/branches/{branchID}/stop-list", deps.AvailabilityHandler.ListStopList)
				admin.Put("/branches/{branchID}/menu-items/{menuItemID}/availability", deps.AvailabilityHandler.UpdateAvailability)
			})
		}
	})

	return r
}
