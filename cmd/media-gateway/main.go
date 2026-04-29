package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmittmann/tint"

	"github.com/yuerchu/media-gateway/internal/api"
	"github.com/yuerchu/media-gateway/internal/callback"
	"github.com/yuerchu/media-gateway/internal/config"
	"github.com/yuerchu/media-gateway/internal/task"
)

func main() {
	// Setup structured logging
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.TimeOnly,
	})))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Set log level
	switch cfg.Log.Level {
	case "debug":
		slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.TimeOnly,
		})))
	case "warn":
		slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelWarn,
			TimeFormat: time.TimeOnly,
		})))
	}

	slog.Info("media-gateway starting", "listen", cfg.Listen)

	// Create temp directory
	if err := os.MkdirAll(cfg.FFmpeg.TempDir, 0o755); err != nil {
		slog.Error("Failed to create temp dir", "path", cfg.FFmpeg.TempDir, "error", err)
		os.Exit(1)
	}

	// Initialize components
	manager := task.NewManager()
	caller := callback.NewCaller()
	worker := task.NewWorker(manager, cfg, caller)

	// Start worker pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Run(ctx)

	// Start task cleanup goroutine (clean completed tasks older than 1 hour)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				removed := manager.CleanupOlderThan(1 * time.Hour)
				if removed > 0 {
					slog.Info("Cleaned up old tasks", "removed", removed)
				}
			}
		}
	}()

	// Setup HTTP server
	router := api.SetupRouter(cfg, manager, worker)

	srv := &http.Server{
		Addr:         cfg.Listen,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down...")

	// Stop HTTP server first
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	// Stop worker pool (drains remaining tasks)
	cancel()

	slog.Info("media-gateway stopped")
}
