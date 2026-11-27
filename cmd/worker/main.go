package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/worker"
	"reel/internal/ai/asr"
	"reel/internal/ai/imagegen"
	"reel/internal/ai/llm"
	"reel/internal/animation"
	"reel/internal/config"
	"reel/internal/logger"
	"reel/internal/media"
	"reel/internal/storage"
	temporalclient "reel/internal/temporal"
	"reel/internal/temporal/activities"
	"reel/internal/temporal/workflows"
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

	// Periodic temp storage TTL cleanup so unpreserved assets don't fill disk.
	if cfg.Storage.TTLHours > 0 {
		ttl := time.Duration(cfg.Storage.TTLHours) * time.Hour
		go runStorageCleanup(ctx, st, ttl, log)
	}

	tc, err := temporalclient.New(cfg.Temporal)
	if err != nil {
		log.Error("failed to connect temporal", "error", err)
		os.Exit(1)
	}
	defer tc.Close()

	workerOpts := worker.Options{
		MaxConcurrentActivityExecutionSize: cfg.Limits.MaxConcurrentJobs,
	}
	w := worker.New(tc, cfg.Temporal.TaskQueue, workerOpts)

	prepActs := &activities.PreprocessActivities{
		Storage:        st,
		Media:          media.NewFFmpegProcessor(cfg.FFmpeg.FFmpegPath, cfg.FFmpeg.FFprobePath),
		MaxDurationSec: cfg.Limits.MaxVideoDurationSec,
	}

	asrProvider, err := asr.NewWithFallback(cfg.AI)
	if err != nil {
		log.Warn("asr provider not initialized", "error", err)
	}
	asrActs := &activities.ASRActivities{
		Storage: st,
		ASR:     asrProvider,
	}
	subActs := &activities.SubtitleActivities{
		Storage: st,
	}

	llmProvider, err := llm.NewWithFallback(cfg.AI)
	if err != nil {
		log.Warn("llm provider not initialized", "error", err)
	}
	storyActs := &activities.StoryActivities{
		Storage: st,
		LLM:     llmProvider,
	}

	imgProvider, err := imagegen.NewWithFallback(cfg.AI)
	if err != nil {
		log.Warn("imagegen provider not initialized", "error", err)
	}
	imgActs := &activities.ImageActivities{
		Storage:        st,
		ImageGen:       imgProvider,
		MaxConcurrency: 3,
	}

	animRenderer, err := animation.NewRenderer("ffmpeg", cfg.FFmpeg.FFmpegPath)
	if err != nil {
		log.Warn("animation renderer not initialized", "error", err)
	}
	animActs := &activities.AnimationActivities{
		Storage:  st,
		Renderer: animRenderer,
	}

	w.RegisterWorkflow(workflows.HelloWorkflow)
	w.RegisterWorkflow(workflows.VideoProcessingWorkflow)
	w.RegisterActivity(activities.HelloActivity)
	w.RegisterActivity(prepActs)
	w.RegisterActivity(asrActs)
	w.RegisterActivity(subActs)
	w.RegisterActivity(storyActs)
	w.RegisterActivity(imgActs)
	w.RegisterActivity(animActs)

	log.Info("starting temporal worker",
		"task_queue", cfg.Temporal.TaskQueue,
		"max_concurrent", cfg.Limits.MaxConcurrentJobs,
	)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Error("temporal worker exited", "error", err)
		os.Exit(1)
	}
}

func runStorageCleanup(ctx context.Context, st storage.Strategy, ttl time.Duration, log interface {
	Info(string, ...any)
	Error(string, ...any)
}) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := st.CleanupExpired(ctx, ttl)
			if err != nil {
				log.Error("temp storage cleanup failed", "error", err)
				continue
			}
			if removed > 0 {
				log.Info("temp storage cleanup completed", "removed_files", removed)
			}
		}
	}
}
