package videogen

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"reel/internal/ai/fallback"
	"reel/internal/config"
)

// Constructor builds a VideoGen Strategy from AIConfig.
type Constructor func(cfg config.AIConfig) (Strategy, error)

var (
	mu       sync.RWMutex
	registry = map[string]Constructor{}
)

// Register registers a VideoGen provider constructor by name.
func Register(name string, ctor Constructor) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(name)] = ctor
}

// New creates the configured primary VideoGen Strategy.
func New(cfg config.AIConfig) (Strategy, error) {
	return build(cfg.VideoGenProvider, cfg)
}

// NewWithFallback creates a VideoGen Strategy with automatic failover if configured.
func NewWithFallback(cfg config.AIConfig) (Strategy, error) {
	primary, err := build(cfg.VideoGenProvider, cfg)
	if err != nil {
		return nil, err
	}
	fbName := strings.TrimSpace(cfg.VideoGenFallbackProvider)
	if fbName == "" || strings.EqualFold(fbName, cfg.VideoGenProvider) {
		return primary, nil
	}
	fb, err := build(fbName, cfg)
	if err != nil {
		return primary, nil
	}
	return &fallbackVideoGen{primary: primary, fallback: fb}, nil
}

func build(name string, cfg config.AIConfig) (Strategy, error) {
	mu.RLock()
	ctor, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("videogen provider %q is not registered", name)
	}
	return ctor(cfg)
}

type fallbackVideoGen struct {
	primary  Strategy
	fallback Strategy
}

func (f *fallbackVideoGen) Name() string {
	return f.primary.Name() + "+" + f.fallback.Name()
}

func (f *fallbackVideoGen) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return fallback.Execute(ctx, f.primary.Name(), f.primary.Generate, f.fallback.Name(), f.fallback.Generate, req)
}
