package media

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// VideoMetadata holds inspected properties of an uploaded video.
type VideoMetadata struct {
	DurationSec float64 `json:"duration_sec"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	HasAudio    bool    `json:"has_audio"`
	Codec       string  `json:"codec"`
	Orientation string  `json:"orientation"` // "landscape" | "portrait" | "square"
}

// JobStage represents the current processing stage of a job.
type JobStage string

const (
	StageUploaded            JobStage = "uploaded"
	StagePreprocessing       JobStage = "preprocessing"
	StageAudioExtracted      JobStage = "audio_extracted"
	StageTranscribing        JobStage = "transcribing"
	StageTranscribed         JobStage = "transcribed"
	StageGeneratingSubtitles JobStage = "generating_subtitles"
	StageSubtitlesReady      JobStage = "subtitles_ready"
	StageFailed              JobStage = "failed"
	StageCancelled           JobStage = "cancelled"
)

// JobStatus represents the live status of a video processing job.
type JobStatus struct {
	JobID           string         `json:"job_id"`
	Stage           JobStage       `json:"stage"`
	Progress        int            `json:"progress"` // 0-100
	InputKey        string         `json:"input_key"`
	AudioKey        string         `json:"audio_key,omitempty"`
	PreviewKey      string         `json:"preview_key,omitempty"`
	TranscriptKey   string         `json:"transcript_key,omitempty"`
	SubtitlesKey    string         `json:"subtitles_key,omitempty"`
	SubtitlesASSKey string         `json:"subtitles_ass_key,omitempty"`
	Metadata        *VideoMetadata `json:"metadata,omitempty"`
	Error           string         `json:"error,omitempty"`
}

var allowedExtensions = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".webm": true,
	".mkv":  true,
	".m4v":  true,
}

// ValidateExtension checks if the filename has a supported video extension.
func ValidateExtension(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExtensions[ext] {
		return fmt.Errorf("unsupported video format %q (allowed: mp4, mov, webm, mkv, m4v)", ext)
	}
	return nil
}

// ValidateVideoHeader inspects the first bytes of a stream to ensure it is a valid video container
// (MP4/MOV 'ftyp', WebM/MKV EBML 0x1A45DFA3, or AVI/RIFF) without reading the whole file.
func ValidateVideoHeader(header []byte) error {
	if len(header) < 12 {
		return errors.New("file too small to be a valid video")
	}
	// ISO Base Media File Format (MP4, MOV, M4V): bytes 4..8 == "ftyp"
	if bytes.Equal(header[4:8], []byte("ftyp")) {
		return nil
	}
	// Matroska / WebM EBML header: 0x1A 0x45 0xDF 0xA3
	if header[0] == 0x1A && header[1] == 0x45 && header[2] == 0xDF && header[3] == 0xA3 {
		return nil
	}
	// MOV wide/moov/mdat atoms at offset 4
	if bytes.Equal(header[4:8], []byte("moov")) || bytes.Equal(header[4:8], []byte("mdat")) || bytes.Equal(header[4:8], []byte("wide")) {
		return nil
	}
	return errors.New("invalid video file signature")
}

// DetectOrientation classifies dimensions into landscape, portrait, or square.
func DetectOrientation(width, height int) string {
	if width > height {
		return "landscape"
	}
	if height > width {
		return "portrait"
	}
	return "square"
}
