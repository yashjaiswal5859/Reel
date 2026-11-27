package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.temporal.io/sdk/temporal"
	"reel/internal/media"
	"reel/internal/storage"
)

var ioBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 64*1024)
		return &b
	},
}

// PreprocessActivities implements Temporal activities for video inspection,
// audio extraction, and preview generation. All fields are immutable and safe for concurrent use.
type PreprocessActivities struct {
	Storage        storage.Strategy
	Media          media.Processor
	MaxDurationSec int
}

// PreprocessInput identifies the job and its uploaded input key.
type PreprocessInput struct {
	JobID    string `json:"job_id"`
	InputKey string `json:"input_key"`
}

// PreprocessOutput holds the generated temporary asset keys.
type PreprocessOutput struct {
	AudioKey   string `json:"audio_key"`
	PreviewKey string `json:"preview_key"`
}

// InspectVideo downloads the input stream to a temp dir, runs ffprobe, and enforces duration limits.
func (a *PreprocessActivities) InspectVideo(ctx context.Context, input PreprocessInput) (*media.VideoMetadata, error) {
	tmpDir, err := os.MkdirTemp("", "reel-inspect-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	localInput := filepath.Join(tmpDir, "input"+filepath.Ext(input.InputKey))
	if err := a.downloadToFile(ctx, input.InputKey, localInput); err != nil {
		return nil, err
	}

	meta, err := a.Media.Probe(ctx, localInput)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("invalid video file: %v", err),
			"InvalidMediaError",
			err,
		)
	}

	if a.MaxDurationSec > 0 && meta.DurationSec > float64(a.MaxDurationSec) {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("video duration %.1fs exceeds limit of %ds", meta.DurationSec, a.MaxDurationSec),
			"InvalidMediaError",
			nil,
		)
	}

	if !meta.HasAudio {
		return nil, temporal.NewNonRetryableApplicationError(
			"video has no audio stream for storytelling",
			"InvalidMediaError",
			nil,
		)
	}

	return meta, nil
}

// ExtractAudioAndPreview extracts normalized 16kHz mono WAV and a 480p preview MP4.
func (a *PreprocessActivities) ExtractAudioAndPreview(ctx context.Context, input PreprocessInput) (*PreprocessOutput, error) {
	tmpDir, err := os.MkdirTemp("", "reel-prep-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	localInput := filepath.Join(tmpDir, "input"+filepath.Ext(input.InputKey))
	if err := a.downloadToFile(ctx, input.InputKey, localInput); err != nil {
		return nil, err
	}

	localAudio := filepath.Join(tmpDir, "audio.wav")
	localPreview := filepath.Join(tmpDir, "preview.mp4")

	// Run audio extraction and preview generation concurrently with coordinated cancellation.
	errCh := make(chan error, 2)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		errCh <- a.Media.ExtractNormalizedAudio(subCtx, localInput, localAudio)
	}()
	go func() {
		errCh <- a.Media.GeneratePreview(subCtx, localInput, localPreview)
	}()

	for i := 0; i < 2; i++ {
		if e := <-errCh; e != nil {
			cancel()
			return nil, e
		}
	}

	audioKey := fmt.Sprintf("jobs/%s/audio.wav", input.JobID)
	if err := a.uploadFromFile(ctx, localAudio, audioKey, "audio/wav"); err != nil {
		return nil, fmt.Errorf("upload audio: %w", err)
	}

	previewKey := fmt.Sprintf("jobs/%s/preview.mp4", input.JobID)
	if err := a.uploadFromFile(ctx, localPreview, previewKey, "video/mp4"); err != nil {
		return nil, fmt.Errorf("upload preview: %w", err)
	}

	return &PreprocessOutput{
		AudioKey:   audioKey,
		PreviewKey: previewKey,
	}, nil
}

func (a *PreprocessActivities) downloadToFile(ctx context.Context, key, dstPath string) error {
	rc, err := a.Storage.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("storage get %s: %w", key, err)
	}
	defer rc.Close()

	f, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create local file: %w", err)
	}
	defer f.Close()

	bufPtr := ioBufPool.Get().(*[]byte)
	defer ioBufPool.Put(bufPtr)

	if _, err := io.CopyBuffer(f, rc, *bufPtr); err != nil {
		return fmt.Errorf("stream download: %w", err)
	}
	return nil
}

func (a *PreprocessActivities) uploadFromFile(ctx context.Context, srcPath, key, contentType string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	return a.Storage.Put(ctx, key, f, info.Size(), contentType)
}
