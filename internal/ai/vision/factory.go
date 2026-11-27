package vision

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"reel/internal/ai/fallback"
	"reel/internal/config"
)

// Constructor builds a Vision Strategy from AIConfig.
type Constructor func(cfg config.AIConfig) (Strategy, error)

var (
	mu       sync.RWMutex
	registry = map[string]Constructor{}
)

// Register registers a Vision provider constructor by name.
func Register(name string, ctor Constructor) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(name)] = ctor
}

// New creates the configured primary Vision Strategy.
func New(cfg config.AIConfig) (Strategy, error) {
	return build(cfg.VisionProvider, cfg)
}

// NewWithFallback creates a Vision Strategy with automatic failover if configured.
func NewWithFallback(cfg config.AIConfig) (Strategy, error) {
	primary, err := build(cfg.VisionProvider, cfg)
	if err != nil {
		return nil, err
	}
	fbName := strings.TrimSpace(cfg.VisionFallbackProvider)
	if fbName == "" || strings.EqualFold(fbName, cfg.VisionProvider) {
		return primary, nil
	}
	fb, err := build(fbName, cfg)
	if err != nil {
		return primary, nil
	}
	return &fallbackVision{primary: primary, fallback: fb}, nil
}

func build(name string, cfg config.AIConfig) (Strategy, error) {
	mu.RLock()
	ctor, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("vision provider %q is not registered", name)
	}
	return ctor(cfg)
}

type fallbackVision struct {
	primary  Strategy
	fallback Strategy
}

func (f *fallbackVision) Name() string {
	return f.primary.Name() + "+" + f.fallback.Name()
}

func (f *fallbackVision) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	return fallback.Execute(ctx, f.primary.Name(), f.primary.Analyze, f.fallback.Name(), f.fallback.Analyze, req)
}
