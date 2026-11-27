package asr

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"reel/internal/ai/fallback"
	"reel/internal/config"
)

// Constructor builds an ASR Strategy from AIConfig.
type Constructor func(cfg config.AIConfig) (Strategy, error)

var (
	mu       sync.RWMutex
	registry = map[string]Constructor{}
)

// Register registers an ASR provider constructor by name.
func Register(name string, ctor Constructor) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(name)] = ctor
}

// New creates the configured primary ASR Strategy.
func New(cfg config.AIConfig) (Strategy, error) {
	return build(cfg.ASRProvider, cfg)
}

// NewWithFallback creates an ASR Strategy that automatically fails over to ASR_FALLBACK_PROVIDER if set.
func NewWithFallback(cfg config.AIConfig) (Strategy, error) {
	primary, err := build(cfg.ASRProvider, cfg)
	if err != nil {
		return nil, err
	}
	fbName := strings.TrimSpace(cfg.ASRFallbackProvider)
	if fbName == "" || strings.EqualFold(fbName, cfg.ASRProvider) {
		return primary, nil
	}
	fb, err := build(fbName, cfg)
	if err != nil {
		return primary, nil
	}
	return &fallbackASR{primary: primary, fallback: fb}, nil
}

func build(name string, cfg config.AIConfig) (Strategy, error) {
	mu.RLock()
	ctor, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("asr provider %q is not registered", name)
	}
	return ctor(cfg)
}

type fallbackASR struct {
	primary  Strategy
	fallback Strategy
}

func (f *fallbackASR) Name() string {
	return f.primary.Name() + "+" + f.fallback.Name()
}

func (f *fallbackASR) Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error) {
	return fallback.Execute(ctx, f.primary.Name(), f.primary.Transcribe, f.fallback.Name(), f.fallback.Transcribe, req)
}
