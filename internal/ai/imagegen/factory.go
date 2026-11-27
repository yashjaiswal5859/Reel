package imagegen

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"reel/internal/ai/fallback"
	"reel/internal/config"
)

// Constructor builds an ImageGen Strategy from AIConfig.
type Constructor func(cfg config.AIConfig) (Strategy, error)

var (
	mu       sync.RWMutex
	registry = map[string]Constructor{}
)

// Register registers an ImageGen provider constructor by name.
func Register(name string, ctor Constructor) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(name)] = ctor
}

// New creates the configured primary ImageGen Strategy.
func New(cfg config.AIConfig) (Strategy, error) {
	return build(cfg.ImageGenProvider, cfg)
}

// NewWithFallback creates an ImageGen Strategy with automatic failover if configured.
func NewWithFallback(cfg config.AIConfig) (Strategy, error) {
	primary, err := build(cfg.ImageGenProvider, cfg)
	if err != nil {
		return nil, err
	}
	fbName := strings.TrimSpace(cfg.ImageGenFallbackProvider)
	if fbName == "" || strings.EqualFold(fbName, cfg.ImageGenProvider) {
		return primary, nil
	}
	fb, err := build(fbName, cfg)
	if err != nil {
		return primary, nil
	}
	return &fallbackImageGen{primary: primary, fallback: fb}, nil
}

func build(name string, cfg config.AIConfig) (Strategy, error) {
	mu.RLock()
	ctor, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("imagegen provider %q is not registered", name)
	}
	return ctor(cfg)
}

type fallbackImageGen struct {
	primary  Strategy
	fallback Strategy
}

func (f *fallbackImageGen) Name() string {
	return f.primary.Name() + "+" + f.fallback.Name()
}

func (f *fallbackImageGen) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	return fallback.Execute(ctx, f.primary.Name(), f.primary.Generate, f.fallback.Name(), f.fallback.Generate, req)
}
