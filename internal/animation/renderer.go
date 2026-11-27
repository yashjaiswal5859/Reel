package animation

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// RendererStrategy defines how a declarative animation Timeline is rendered into a video file.
// Implementations can use procedural FFmpeg filters or an external Remotion sidecar.
type RendererStrategy interface {
	Name() string
	RenderTimeline(ctx context.Context, timeline *Timeline, sceneImagePaths map[string]string, outputPath string) error
}

// RendererConstructor builds a RendererStrategy.
type RendererConstructor func(binaryPath string) RendererStrategy

var (
	regMu            sync.RWMutex
	rendererRegistry = map[string]RendererConstructor{}
)

// RegisterRenderer registers an animation renderer strategy by name.
func RegisterRenderer(name string, ctor RendererConstructor) {
	regMu.Lock()
	defer regMu.Unlock()
	rendererRegistry[strings.ToLower(name)] = ctor
}

// NewRenderer creates a RendererStrategy by name (defaults to "ffmpeg").
func NewRenderer(name, binaryPath string) (RendererStrategy, error) {
	if strings.TrimSpace(name) == "" {
		name = "ffmpeg"
	}
	regMu.RLock()
	ctor, ok := rendererRegistry[strings.ToLower(strings.TrimSpace(name))]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("animation renderer %q is not registered", name)
	}
	return ctor(binaryPath), nil
}
