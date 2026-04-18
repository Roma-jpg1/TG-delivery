package middleware

import (
	"net/http"
	"time"

	"TG-delivery/internal/observability"
)

func HTTPMetrics(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if metrics == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(recorder, r)
			metrics.ObserveRequest(r.Method, r.URL.Path, recorder.statusCode, time.Since(start))
		})
	}
}
