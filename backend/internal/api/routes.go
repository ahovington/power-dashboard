package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handler, allowedOrigin string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS: env-driven allowed origin, explicit rather than wildcard.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{allowedOrigin},
		AllowedMethods: []string{"GET", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	// Liveness: is the process running?
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Readiness: is the DB reachable?
	r.Get("/ready", h.Ready)

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		// SSE: no TimeoutHandler — connection is intentionally long-lived
		r.Get("/events", h.ServeEvents)

		// All other endpoints: 30s write timeout
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.TimeoutHandler(next, 30*time.Second, `{"error":"request timeout"}`)
			})
			r.Get("/power/status", h.GetCurrentStatus)
			r.Get("/power/history", h.GetHistory)
		})
	})

	return r
}
