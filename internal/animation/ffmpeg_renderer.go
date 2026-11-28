package animation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	RegisterRenderer("ffmpeg", func(binaryPath string) RendererStrategy {
		return NewFFmpegRenderer(binaryPath)
	})
}

// FFmpegRenderer implements RendererStrategy using procedural FFmpeg zoompan,
// environment overlays, and xfade scene transitions.
type FFmpegRenderer struct {
	ffmpegPath string
}

// NewFFmpegRenderer creates an FFmpeg-backed animation renderer.
func NewFFmpegRenderer(ffmpegPath string) *FFmpegRenderer {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &FFmpegRenderer{ffmpegPath: ffmpegPath}
}

func (r *FFmpegRenderer) Name() string { return "ffmpeg" }

// RenderTimeline renders each scene clip with camera/environment motion and stitches
// them with xfade transitions into outputPath.
func (r *FFmpegRenderer) RenderTimeline(ctx context.Context, tl *Timeline, sceneImagePaths map[string]string, outputPath string) error {
	if tl == nil || len(tl.Scenes) == 0 {
		return fmt.Errorf("empty animation timeline")
	}

	tmpDir, err := os.MkdirTemp("", "reel-anim-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	clipPaths := make([]string, 0, len(tl.Scenes))
	for i, sc := range tl.Scenes {
		imgPath, ok := sceneImagePaths[sc.SceneID]
		if !ok || imgPath == "" {
			return fmt.Errorf("missing image path for scene %s", sc.SceneID)
		}

		clipPath := filepath.Join(tmpDir, fmt.Sprintf("scene_%03d.mp4", i))
		filter := BuildSceneFilter(sc, tl.Width, tl.Height, tl.FPS)
		durStr := fmt.Sprintf("%.3f", sc.DurationSec)

		if err := r.runFFmpeg(ctx,
			"-y",
			"-i", imgPath,
			"-vf", filter,
			"-t", durStr,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-r", fmt.Sprintf("%d", tl.FPS),
			clipPath,
		); err != nil {
			return fmt.Errorf("render scene %s: %w", sc.SceneID, err)
		}
		clipPaths = append(clipPaths, clipPath)
	}

	if len(clipPaths) == 1 {
		return copyFile(clipPaths[0], outputPath)
	}

	return r.stitchWithTransitions(ctx, tl.Scenes, clipPaths, outputPath)
}

// BuildSceneFilter constructs the FFmpeg video filtergraph for camera motion and environment effects.
func BuildSceneFilter(sc ScenePlan, width, height, fps int) string {
	if width <= 0 {
		width = 1080
	}
	if height <= 0 {
		height = 1080
	}
	if fps <= 0 {
		fps = 30
	}

	frames := int(sc.DurationSec * float64(fps))
	if frames < 1 {
		frames = fps * 2
	}

	// Upscale input before zoompan for smooth subpixel motion, then crop/scale to target size.
	var zp string
	switch sc.Camera {
	case CamZoomIn:
		zp = fmt.Sprintf("zoompan=z='min(zoom+0.0015,1.25)':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=%d:s=%dx%d:fps=%d", frames, width, height, fps)
	case CamZoomOut:
		zp = fmt.Sprintf("zoompan=z='if(lte(zoom,1.0),1.25,max(1.001,zoom-0.0015))':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=%d:s=%dx%d:fps=%d", frames, width, height, fps)
	case CamPanLeft:
		zp = fmt.Sprintf("zoompan=z='1.15':x='(iw-iw/zoom)*(1-on/%d)':y='ih/2-(ih/zoom/2)':d=%d:s=%dx%d:fps=%d", frames, frames, width, height, fps)
	case CamPanRight:
		zp = fmt.Sprintf("zoompan=z='1.15':x='(iw-iw/zoom)*(on/%d)':y='ih/2-(ih/zoom/2)':d=%d:s=%dx%d:fps=%d", frames, frames, width, height, fps)
	case CamPanUp:
		zp = fmt.Sprintf("zoompan=z='1.15':x='iw/2-(iw/zoom/2)':y='(ih-ih/zoom)*(1-on/%d)':d=%d:s=%dx%d:fps=%d", frames, frames, width, height, fps)
	case CamPanDown:
		zp = fmt.Sprintf("zoompan=z='1.15':x='iw/2-(iw/zoom/2)':y='(ih-ih/zoom)*(on/%d)':d=%d:s=%dx%d:fps=%d", frames, frames, width, height, fps)
	case CamShake:
		zp = fmt.Sprintf("zoompan=z='1.1':x='iw/2-(iw/zoom/2)+8*sin(on*1.2)':y='ih/2-(ih/zoom/2)+6*cos(on*1.5)':d=%d:s=%dx%d:fps=%d", frames, width, height, fps)
	default:
		zp = fmt.Sprintf("zoompan=z='1.02':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=%d:s=%dx%d:fps=%d", frames, width, height, fps)
	}

	filters := []string{
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", width*2, height*2, width*2, height*2),
		zp,
	}

	switch sc.Environment {
	case EnvFireGlow:
		filters = append(filters, "eq=brightness=0.03*sin(t*5):saturation=1.15")
	case EnvRain:
		filters = append(filters, "eq=brightness=-0.04:contrast=1.08")
	case EnvSnow:
		filters = append(filters, "eq=brightness=0.05:saturation=0.85")
	}

	filters = append(filters, "format=yuv420p")
	return strings.Join(filters, ",")
}

// MapXfadeTransition maps domain TransitionType to FFmpeg xfade transition names.
func MapXfadeTransition(t TransitionType) string {
	switch t {
	case TransSlideLeft:
		return "slideleft"
	case TransSlideRight:
		return "slideright"
	case TransWipe:
		return "wipeleft"
	case TransZoom:
		return "zoomin"
	default:
		return "fade"
	}
}

func (r *FFmpegRenderer) stitchWithTransitions(ctx context.Context, scenes []ScenePlan, clipPaths []string, outputPath string) error {
	args := []string{"-y"}
	for _, p := range clipPaths {
		args = append(args, "-i", p)
	}

	var fc strings.Builder
	offset := scenes[0].DurationSec
	lastLabel := "[0:v]"

	for i := 1; i < len(clipPaths); i++ {
		tdur := scenes[i].TransitionDur
		if tdur <= 0.05 {
			tdur = 0.3
		}
		if offset > tdur {
			offset -= tdur
		}
		outLabel := fmt.Sprintf("[v%d]", i)
		if i == len(clipPaths)-1 {
			outLabel = "[vout]"
		}
		xfadeName := MapXfadeTransition(scenes[i].Transition)
		fmt.Fprintf(&fc, "%s[%d:v]xfade=transition=%s:duration=%.3f:offset=%.3f%s",
			lastLabel, i, xfadeName, tdur, offset, outLabel)
		if i < len(clipPaths)-1 {
			fc.WriteString(";")
		}
		offset += scenes[i].DurationSec
		lastLabel = outLabel
	}

	args = append(args,
		"-filter_complex", fc.String(),
		"-map", "[vout]",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		outputPath,
	)

	return r.runFFmpeg(ctx, args...)
}

func (r *FFmpegRenderer) runFFmpeg(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, r.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{buf: &stderr, max: 8 * 1024}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w (stderr: %s)", err, stderr.String())
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

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
