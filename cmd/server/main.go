package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Priya8975/webhook-delivery-system/internal/api"
	"github.com/Priya8975/webhook-delivery-system/internal/config"
	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/Priya8975/webhook-delivery-system/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL
	pgStore, err := store.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()
	logger.Info("connected to PostgreSQL")

	// Run database migrations
	if err := pgStore.RunMigrations(ctx, "migrations"); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations applied")

	// Initialize Redis
	redisStore, err := store.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisStore.Close()
	logger.Info("connected to Redis")

	// Initialize fan-out engine
	fanout := engine.NewFanOutEngine(pgStore, redisStore, logger)

	// Start worker pool and dispatcher
	deliverer := worker.NewDeliverer(pgStore, logger)
	pool := worker.NewPool(cfg.NumWorkers, deliverer, logger)
	pool.Start(ctx)

	dispatcher := worker.NewDispatcher(redisStore.Client(), pool, logger)
	go dispatcher.Start(ctx)

	// Setup router
	router := api.NewRouter(pgStore, fanout)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Cancel context to stop dispatcher and workers
	cancel()

	// Stop worker pool (waits for in-flight deliveries)
	pool.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
