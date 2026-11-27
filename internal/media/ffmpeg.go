package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Processor defines media inspection and transcoding operations.
type Processor interface {
	Probe(ctx context.Context, inputPath string) (*VideoMetadata, error)
	ExtractNormalizedAudio(ctx context.Context, inputPath, outputPath string) error
	GeneratePreview(ctx context.Context, inputPath, outputPath string) error
}

// FFmpegProcessor executes ffprobe and ffmpeg subprocesses safely with bounded stderr.
type FFmpegProcessor struct {
	ffmpegPath  string
	ffprobePath string
}

// NewFFmpegProcessor creates a Processor using the configured binary paths.
func NewFFmpegProcessor(ffmpegPath, ffprobePath string) *FFmpegProcessor {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	return &FFmpegProcessor{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
	}
}

type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
}

// Probe inspects a video file using ffprobe and returns structured VideoMetadata.
func (p *FFmpegProcessor) Probe(ctx context.Context, inputPath string) (*VideoMetadata, error) {
	cmd := exec.CommandContext(ctx, p.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	var stdout bytes.Buffer
	stderr := newLimitedBuffer(8 * 1024)
	cmd.Stdout = &stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w (stderr: %s)", err, stderr.String())
	}

	return ParseFFprobeJSON(stdout.Bytes())
}

// ParseFFprobeJSON parses ffprobe JSON output into VideoMetadata.
func ParseFFprobeJSON(data []byte) (*VideoMetadata, error) {
	var raw ffprobeOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe json: %w", err)
	}

	meta := &VideoMetadata{}
	if raw.Format.Duration != "" {
		dur, err := strconv.ParseFloat(raw.Format.Duration, 64)
		if err == nil {
			meta.DurationSec = dur
		}
	}

	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			if meta.Width == 0 {
				meta.Width = s.Width
				meta.Height = s.Height
				meta.Codec = s.CodecName
				meta.FPS = parseFrameRate(s.RFrameRate)
				meta.Orientation = DetectOrientation(s.Width, s.Height)
			}
		case "audio":
			meta.HasAudio = true
		}
	}

	if meta.Width == 0 || meta.Height == 0 {
		return nil, fmt.Errorf("no valid video stream found")
	}
	return meta, nil
}

// ExtractNormalizedAudio extracts mono 16kHz PCM WAV with loudness normalization for ASR.
func (p *FFmpegProcessor) ExtractNormalizedAudio(ctx context.Context, inputPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		"-af", "loudnorm",
		outputPath,
	}
	return p.runFFmpeg(ctx, args)
}

// GeneratePreview creates a lightweight 480p H.264/AAC MP4 preview.
func (p *FFmpegProcessor) GeneratePreview(ctx context.Context, inputPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", "scale=-2:480",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "28",
		"-c:a", "aac",
		"-b:a", "96k",
		"-movflags", "+faststart",
		outputPath,
	}
	return p.runFFmpeg(ctx, args)
}

func (p *FFmpegProcessor) runFFmpeg(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, p.ffmpegPath, args...)
	stderr := newLimitedBuffer(8 * 1024)
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

func parseFrameRate(rate string) float64 {
	parts := strings.Split(rate, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			return num / den
		}
	}
	f, err := strconv.ParseFloat(rate, 64)
	if err != nil {
		return 0
	}
	return f
}

// limitedBuffer caps memory usage for subprocess stderr output.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{max: max}
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	origLen := len(p)
	if lb.buf.Len() >= lb.max {
		return origLen, nil
	}
	remaining := lb.max - lb.buf.Len()
	if len(p) > remaining {
		p = p[:remaining]
	}
	_, err := lb.buf.Write(p)
	return origLen, err
}

func (lb *limitedBuffer) String() string {
	return lb.buf.String()
}
