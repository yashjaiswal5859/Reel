package composition

import (
	"fmt"
	"math"

	"reel/internal/media"
)

// ValidationReport summarizes final MP4 quality checks (REQ-064).
type ValidationReport struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// ValidateFinalVideo verifies that the rendered MP4 matches expected duration, resolution, and audio presence.
func ValidateFinalVideo(finalMeta *media.VideoMetadata, expectedDurationSec float64, cfg ExportConfig, fileSizeBytes int64) ValidationReport {
	report := ValidationReport{Valid: true}

	if fileSizeBytes <= 0 {
		report.Valid = false
		report.Errors = append(report.Errors, "final video file is empty")
		return report
	}
	if finalMeta == nil {
		report.Valid = false
		report.Errors = append(report.Errors, "missing final video metadata")
		return report
	}

	expW, expH := TargetDimensions(cfg.AspectRatio, cfg.Quality)
	if finalMeta.Width != expW || finalMeta.Height != expH {
		report.Valid = false
		report.Errors = append(report.Errors, fmt.Sprintf("resolution %dx%d does not match expected %dx%d",
			finalMeta.Width, finalMeta.Height, expW, expH))
	}

	if !finalMeta.HasAudio {
		report.Valid = false
		report.Errors = append(report.Errors, "final video is missing audio stream")
	}

	if expectedDurationSec > 0 && math.Abs(finalMeta.DurationSec-expectedDurationSec) > 1.0 {
		report.Valid = false
		report.Errors = append(report.Errors, fmt.Sprintf("duration %.2fs drifts from original %.2fs",
			finalMeta.DurationSec, expectedDurationSec))
	}

	return report
}
