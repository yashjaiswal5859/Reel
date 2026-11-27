package composition

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Compositor combines presenter video, story animation, and subtitles into the final MP4.
type Compositor interface {
	Compose(ctx context.Context, req ComposeRequest) error
}

// ComposeRequest specifies local file paths and export configuration.
type ComposeRequest struct {
	PresenterVideoPath string
	AnimationVideoPath string
	SubtitlesASSPath   string
	OutputPath         string
	Config             ExportConfig
}

// FFmpegCompositor executes FFmpeg split-screen composition with bounded stderr.
type FFmpegCompositor struct {
	ffmpegPath string
}

// NewFFmpegCompositor creates an FFmpegCompositor.
func NewFFmpegCompositor(ffmpegPath string) *FFmpegCompositor {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &FFmpegCompositor{ffmpegPath: ffmpegPath}
}

func (c *FFmpegCompositor) Compose(ctx context.Context, req ComposeRequest) error {
	absPres, err := filepath.Abs(req.PresenterVideoPath)
	if err != nil {
		return err
	}
	absAnim, err := filepath.Abs(req.AnimationVideoPath)
	if err != nil {
		return err
	}
	absOut, err := filepath.Abs(req.OutputPath)
	if err != nil {
		return err
	}

	layoutStrat := NewLayoutStrategy(req.Config.Layout, req.Config.AspectRatio)
	layout := layoutStrat.Compute(req.Config)

	assBase := ""
	workDir := filepath.Dir(absOut)
	if req.SubtitlesASSPath != "" {
		assBase = filepath.Base(req.SubtitlesASSPath)
		workDir = filepath.Dir(req.SubtitlesASSPath)
	}

	filterComplex := BuildSplitFilter(layout, req.Config.PresenterFirst, assBase)

	fps := req.Config.FPS
	if fps <= 0 {
		fps = 30
	}
	crf := req.Config.CRF
	if crf <= 0 {
		crf = 20
	}

	// Stream 0 = presenter (source of truth for audio & duration), Stream 1 = animation (stream_loop -1 + shortest).
	args := []string{
		"-y",
		"-i", absPres,
		"-stream_loop", "-1",
		"-i", absAnim,
		"-filter_complex", filterComplex,
		"-map", "[vout]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", fmt.Sprintf("%d", crf),
		"-r", fmt.Sprintf("%d", fps),
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		"-movflags", "+faststart",
		absOut,
	}

	cmd := exec.CommandContext(ctx, c.ffmpegPath, args...)
	cmd.Dir = workDir
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{buf: &stderr, max: 8 * 1024}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg compose failed: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

type limitedWriter struct {
	buf *bytes.Buffer
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	orig := len(p)
	if w.buf.Len() >= w.max {
		return orig, nil
	}
	rem := w.max - w.buf.Len()
	if len(p) > rem {
		p = p[:rem]
	}
	_, err := w.buf.Write(p)
	return orig, err
}
