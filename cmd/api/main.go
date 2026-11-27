package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"reel/internal/ai/imagegen"
	"reel/internal/api"
	"reel/internal/config"
	"reel/internal/logger"
	"reel/internal/storage"
	temporalclient "reel/internal/temporal"
)

func main() {
	envFile := ".env"
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		envFile = ""
	}

	cfg, err := config.Load(envFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.App.LogLevel, cfg.App.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		log.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}

	tc, err := temporalclient.New(cfg.Temporal)
	if err != nil {
		log.Error("failed to connect temporal", "error", err)
		os.Exit(1)
	}
	defer tc.Close()

	imgProvider, err := imagegen.NewWithFallback(cfg.AI)
	if err != nil {
		log.Warn("imagegen provider not initialized for scene regeneration", "error", err)
	}

	jobSvc := api.NewTemporalJobService(tc, cfg.Temporal.TaskQueue)
	router := api.NewRouter(api.RouterDeps{
		Logger:           log,
		Env:              cfg.App.Env,
		Storage:          st,
		Jobs:             jobSvc,
		ImageGen:         imgProvider,
		MaxFileSizeMB:    int64(cfg.Limits.MaxFileSizeMB),
		MaxRegenAttempts: cfg.Limits.MaxRegenAttempts,
		DefaultASR:       cfg.AI.ASRProvider,
		DefaultLLM:       cfg.AI.LLMProvider,
		DefaultImageGen:  cfg.AI.ImageGenProvider,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("starting api server", "addr", srv.Addr, "env", cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("api server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down api server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("api server forced shutdown", "error", err)
	}
}
