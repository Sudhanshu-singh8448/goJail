// Package main is the entry point for goboxd — a Go HTTP service that
// runs untrusted code inside nsjail sandboxes and returns per-test results.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nsjail-server/gojail/internal/api"
	"github.com/nsjail-server/gojail/internal/config"
	"github.com/nsjail-server/gojail/internal/logger"
	"github.com/nsjail-server/gojail/internal/sandbox"
	"github.com/nsjail-server/gojail/internal/worker"
)

func main() {
	// Initialize structured JSON logging
	logger.Init()

	// Determine config directory
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "config"
	}

	// Load server config
	serverCfg, err := config.LoadServerConfig(filepath.Join(configDir, "server.yaml"))
	if err != nil {
		slog.Error("failed to load server config", "error", err)
		os.Exit(1)
	}

	// Override port from env if set
	if port := os.Getenv("PORT"); port != "" {
		fmt.Sscanf(port, "%d", &serverCfg.Server.Port)
	}

	// Override max concurrent jobs from env if set
	if mcj := os.Getenv("MAX_CONCURRENT_JOBS"); mcj != "" {
		fmt.Sscanf(mcj, "%d", &serverCfg.Server.MaxConcurrentJobs)
	}
	if serverCfg.Server.MaxConcurrentJobs == 0 {
		serverCfg.Server.MaxConcurrentJobs = runtime.NumCPU()
	}

	// Load language configs
	langs, err := config.LoadLanguages(filepath.Join(configDir, "languages.yaml"))
	if err != nil {
		slog.Error("failed to load languages config", "error", err)
		os.Exit(1)
	}

	slog.Info("languages loaded", "count", len(langs))
	for _, lang := range langs {
		slog.Info("registered language",
			"id", lang.ID,
			"name", lang.Name,
			"needs_compilation", lang.NeedsCompilation(),
		)
	}

	// Ensure jail directory exists
	if err := os.MkdirAll(serverCfg.Server.JailDir, 0755); err != nil {
		slog.Error("failed to create jail directory", "path", serverCfg.Server.JailDir, "error", err)
		os.Exit(1)
	}

	// Sweep orphan jails from previous runs (security fix #7)
	sweepAge := time.Duration(serverCfg.Server.OrphanSweepMinutes) * time.Minute
	swept := sandbox.SweepOrphanJails(serverCfg.Server.JailDir, sweepAge)
	if swept > 0 {
		slog.Info("swept orphan jail directories", "count", swept)
	}

	// Start periodic orphan sweep
	go func() {
		ticker := time.NewTicker(sweepAge)
		defer ticker.Stop()
		for range ticker.C {
			n := sandbox.SweepOrphanJails(serverCfg.Server.JailDir, sweepAge)
			if n > 0 {
				slog.Info("periodic orphan sweep", "swept", n)
			}
		}
	}()

	// Create worker pool
	pool := worker.NewPool(serverCfg.Server.MaxConcurrentJobs)
	slog.Info("worker pool created", "max_concurrent", serverCfg.Server.MaxConcurrentJobs)

	// Create API server
	srv := api.NewServer(&serverCfg.Server, langs, pool)

	// Set up chi router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	// Routes
	r.Get("/healthz", srv.HandleHealthz)
	r.Get("/readyz", srv.HandleReadyz)
	r.Get("/info", srv.HandleInfo)
	r.Post("/run", srv.HandleRun)

	// Start server
	addr := fmt.Sprintf("0.0.0.0:%d", serverCfg.Server.Port)
	slog.Info("starting goboxd", "address", addr, "version", api.Version)

	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
