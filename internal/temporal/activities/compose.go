package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.temporal.io/sdk/temporal"
	"reel/internal/composition"
	"reel/internal/media"
	"reel/internal/storage"
)

// ComposeActivities implements Temporal activities for final split-screen composition and validation.
type ComposeActivities struct {
	Storage    storage.Strategy
	Compositor composition.Compositor
	Media      media.Processor
}

// ComposeInput specifies all assets needed to render the final video.
type ComposeInput struct {
	JobID             string                   `json:"job_id"`
	InputKey          string                   `json:"input_key"`
	AnimationVideoKey string                   `json:"animation_video_key"`
	SubtitlesASSKey   string                   `json:"subtitles_ass_key"`
	ExpectedDuration  float64                  `json:"expected_duration_sec"`
	ExportConfig      composition.ExportConfig `json:"export_config"`
}

// ComposeOutput returns the stored final MP4 key and validation report.
type ComposeOutput struct {
	FinalVideoKey string                       `json:"final_video_key"`
	Validation    composition.ValidationReport `json:"validation"`
}

// ComposeFinalVideo composites presenter + animation + subtitles into final.mp4 and validates it.
func (a *ComposeActivities) ComposeFinalVideo(ctx context.Context, input ComposeInput) (*ComposeOutput, error) {
	if a.Compositor == nil {
		return nil, temporal.NewNonRetryableApplicationError("compositor not configured", "ConfigError", nil)
	}

	tmpDir, err := os.MkdirTemp("", "reel-compose-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	localPres := filepath.Join(tmpDir, "presenter"+filepath.Ext(input.InputKey))
	if err := a.downloadToFile(ctx, input.InputKey, localPres); err != nil {
		return nil, fmt.Errorf("download presenter: %w", err)
	}

	localAnim := filepath.Join(tmpDir, "animation.mp4")
	if err := a.downloadToFile(ctx, input.AnimationVideoKey, localAnim); err != nil {
		return nil, fmt.Errorf("download animation: %w", err)
	}

	localASS := ""
	if input.SubtitlesASSKey != "" {
		localASS = filepath.Join(tmpDir, "subtitles.ass")
		if err := a.downloadToFile(ctx, input.SubtitlesASSKey, localASS); err != nil {
			return nil, fmt.Errorf("download subtitles: %w", err)
		}
	}

	localOut := filepath.Join(tmpDir, "final.mp4")
	if err := a.Compositor.Compose(ctx, composition.ComposeRequest{
		PresenterVideoPath: localPres,
		AnimationVideoPath: localAnim,
		SubtitlesASSPath:   localASS,
		OutputPath:         localOut,
		Config:             input.ExportConfig,
	}); err != nil {
		return nil, err
	}

	info, err := os.Stat(localOut)
	if err != nil {
		return nil, fmt.Errorf("stat final video: %w", err)
	}

	var validation composition.ValidationReport
	if a.Media != nil {
		finalMeta, err := a.Media.Probe(ctx, localOut)
		if err != nil {
			return nil, fmt.Errorf("probe final video: %w", err)
		}
		validation = composition.ValidateFinalVideo(finalMeta, input.ExpectedDuration, input.ExportConfig, info.Size())
	} else {
		validation = composition.ValidationReport{Valid: info.Size() > 0}
	}

	finalKey := fmt.Sprintf("jobs/%s/final.mp4", input.JobID)
	if err := a.uploadFromFile(ctx, localOut, finalKey, "video/mp4"); err != nil {
		return nil, fmt.Errorf("upload final video: %w", err)
	}

	return &ComposeOutput{
		FinalVideoKey: finalKey,
		Validation:    validation,
	}, nil
}

func (a *ComposeActivities) downloadToFile(ctx context.Context, key, dstPath string) error {
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

func (a *ComposeActivities) uploadFromFile(ctx context.Context, srcPath, key, contentType string) error {
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
