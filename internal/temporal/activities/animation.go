package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.temporal.io/sdk/temporal"
	"reel/internal/animation"
	"reel/internal/storage"
)

// AnimationActivities implements Temporal activities for planning and rendering story animations.
type AnimationActivities struct {
	Storage  storage.Strategy
	Renderer animation.RendererStrategy
}

// AnimationInput specifies the job, storyboard, and target animation panel dimensions.
type AnimationInput struct {
	JobID         string `json:"job_id"`
	StoryboardKey string `json:"storyboard_key"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	FPS           int    `json:"fps,omitempty"`
}

// AnimationOutput returns the declarative timeline JSON key and rendered animation MP4 key.
type AnimationOutput struct {
	TimelineKey       string `json:"timeline_key"`
	AnimationVideoKey string `json:"animation_video_key"`
}

// RenderAnimation plans the declarative animation timeline and renders it via RendererStrategy.
func (a *AnimationActivities) RenderAnimation(ctx context.Context, input AnimationInput) (*AnimationOutput, error) {
	if a.Renderer == nil {
		return nil, temporal.NewNonRetryableApplicationError("animation renderer not configured", "ConfigError", nil)
	}

	rc, err := a.Storage.Get(ctx, input.StoryboardKey)
	if err != nil {
		return nil, fmt.Errorf("get storyboard %s: %w", input.StoryboardKey, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read storyboard: %w", err)
	}

	var sbDoc StoredStoryboard
	if err := json.Unmarshal(raw, &sbDoc); err != nil {
		return nil, fmt.Errorf("decode storyboard: %w", err)
	}

	tl, err := animation.PlanTimeline(input.JobID, sbDoc.Storyboard, input.Width, input.Height, input.FPS)
	if err != nil {
		return nil, fmt.Errorf("plan animation: %w", err)
	}

	tlBytes, err := json.Marshal(tl)
	if err != nil {
		return nil, fmt.Errorf("marshal timeline: %w", err)
	}
	tlKey := fmt.Sprintf("jobs/%s/animation_timeline.json", input.JobID)
	if err := a.Storage.Put(ctx, tlKey, bytes.NewReader(tlBytes), int64(len(tlBytes)), "application/json"); err != nil {
		return nil, fmt.Errorf("store timeline: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "reel-render-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	sceneImgPaths := make(map[string]string, len(tl.Scenes))
	for _, sc := range tl.Scenes {
		if sc.ImageKey == "" {
			return nil, fmt.Errorf("scene %s has no generated image_key", sc.SceneID)
		}
		localImg := filepath.Join(tmpDir, sc.SceneID+filepath.Ext(sc.ImageKey))
		if err := a.downloadToFile(ctx, sc.ImageKey, localImg); err != nil {
			return nil, fmt.Errorf("download scene %s image: %w", sc.SceneID, err)
		}
		sceneImgPaths[sc.SceneID] = localImg
	}

	localOut := filepath.Join(tmpDir, "animation.mp4")
	if err := a.Renderer.RenderTimeline(ctx, tl, sceneImgPaths, localOut); err != nil {
		return nil, fmt.Errorf("render animation: %w", err)
	}

	outKey := fmt.Sprintf("jobs/%s/animation.mp4", input.JobID)
	if err := a.uploadFromFile(ctx, localOut, outKey, "video/mp4"); err != nil {
		return nil, fmt.Errorf("upload animation: %w", err)
	}

	return &AnimationOutput{
		TimelineKey:       tlKey,
		AnimationVideoKey: outKey,
	}, nil
}

func (a *AnimationActivities) downloadToFile(ctx context.Context, key, dstPath string) error {
	rc, err := a.Storage.Get(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	bufPtr := ioBufPool.Get().(*[]byte)
	defer ioBufPool.Put(bufPtr)
	_, err = io.CopyBuffer(f, rc, *bufPtr)
	return err
}

func (a *AnimationActivities) uploadFromFile(ctx context.Context, srcPath, key, contentType string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	return a.Storage.Put(ctx, key, f, info.Size(), contentType)
}
