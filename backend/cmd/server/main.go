package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/power-dashboard/internal/api"
	"github.com/yourusername/power-dashboard/internal/config"
	"github.com/yourusername/power-dashboard/internal/model"
	"github.com/yourusername/power-dashboard/internal/repository"
	"github.com/yourusername/power-dashboard/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("startup: config", "error", err)
		os.Exit(1)
	}

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	// Default pool MaxConns = max(4, numCPU). Fine for single-household development.
	// To tune: use pgxpool.ParseConfig() and set config.MaxConns before NewWithConfig().
	if err != nil {
		slog.Error("startup: db connect", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("startup: db ping", "error", err)
		os.Exit(1)
	}

	m, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		slog.Error("startup: migration init", "error", err)
		os.Exit(1)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		slog.Error("startup: migration up", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: migrations applied")

	readingRepo := repository.NewReadingRepository(db)
	powerSvc := service.NewPowerService(readingRepo)
	hub := api.NewHub()
	handler := api.NewHandler(powerSvc, hub, db)
	router := api.NewRouter(handler, cfg.CORSAllowedOrigin)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	eventBus := make(chan model.PowerEvent, 64)

	// Bridge eventBus → hub fan-out
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventBus:
				hub.Broadcast(event)
			}
		}
	}()

	// Start one IngestionService per configured provider
	for _, p := range service.BuildProviders(time.NewTicker(cfg.PollInterval).C) {
		svc := service.NewIngestionService(p.Adapter, readingRepo, eventBus, p.DeviceID, p.Trigger)
		go svc.RunPoller(ctx)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second, // SSE handler is exempt — it's wrapped by the chi group, not the server timeout
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("startup: listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("shutdown: signal received, draining...")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown: http server", "error", err)
	}
	slog.Info("shutdown: complete")
}
