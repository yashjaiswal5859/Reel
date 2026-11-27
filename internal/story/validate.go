package story

import (
	"fmt"
	"strings"
)

// ValidationIssue describes a storyboard or timeline defect (REQ-061, REQ-063).
type ValidationIssue struct {
	Code       string  `json:"code"`
	Severity   string  `json:"severity"` // "error" | "warning"
	SceneIndex int     `json:"scene_index"`
	Timestamp  float64 `json:"timestamp,omitempty"`
	Message    string  `json:"message"`
}

// QualityReport summarizes storyboard validation results.
type QualityReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidateStoryboard verifies chronological order, valid durations, character references, and visual types.
func ValidateStoryboard(sb *Storyboard, videoDurationSec float64) QualityReport {
	report := QualityReport{Valid: true}
	if sb == nil || len(sb.Scenes) == 0 {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{
			Code:     "EMPTY_STORYBOARD",
			Severity: "error",
			Message:  "storyboard contains no scenes",
		})
		return report
	}

	knownChars := make(map[string]bool, len(sb.CharacterBible))
	for _, c := range sb.CharacterBible {
		if c.ID != "" {
			knownChars[c.ID] = true
		}
	}

	var prevEnd float64
	for i, sc := range sb.Scenes {
		if sc.StartSec < 0 || sc.EndSec <= sc.StartSec {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{
				Code:       "INVALID_SCENE_TIMING",
				Severity:   "error",
				SceneIndex: sc.Index,
				Timestamp:  sc.StartSec,
				Message:    fmt.Sprintf("scene %d has invalid timing [%.2f, %.2f]", i+1, sc.StartSec, sc.EndSec),
			})
		}

		if sc.StartSec < prevEnd-0.01 {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{
				Code:       "SCENE_OVERLAP",
				Severity:   "error",
				SceneIndex: sc.Index,
				Timestamp:  sc.StartSec,
				Message:    fmt.Sprintf("scene %d starts at %.2fs before previous scene end %.2fs", i+1, sc.StartSec, prevEnd),
			})
		}

		if videoDurationSec > 0 && sc.EndSec > videoDurationSec+0.5 {
			report.Issues = append(report.Issues, ValidationIssue{
				Code:       "SCENE_EXCEEDS_VIDEO",
				Severity:   "warning",
				SceneIndex: sc.Index,
				Timestamp:  sc.EndSec,
				Message:    fmt.Sprintf("scene %d ends at %.2fs beyond video duration %.2fs", i+1, sc.EndSec, videoDurationSec),
			})
		}

		if strings.TrimSpace(sc.Description) == "" {
			report.Valid = false
			report.Issues = append(report.Issues, ValidationIssue{
				Code:       "EMPTY_SCENE_DESCRIPTION",
				Severity:   "error",
				SceneIndex: sc.Index,
				Message:    fmt.Sprintf("scene %d is missing visual description", i+1),
			})
		}

		if !IsValidVisualType(sc.VisualType) {
			report.Issues = append(report.Issues, ValidationIssue{
				Code:       "UNKNOWN_VISUAL_TYPE",
				Severity:   "warning",
				SceneIndex: sc.Index,
				Message:    fmt.Sprintf("scene %d has unrecognized visual_type %q", i+1, sc.VisualType),
			})
		}

		for _, cid := range sc.CharacterIDs {
			if !knownChars[cid] {
				report.Issues = append(report.Issues, ValidationIssue{
					Code:       "UNKNOWN_CHARACTER_REF",
					Severity:   "warning",
					SceneIndex: sc.Index,
					Message:    fmt.Sprintf("scene %d references unknown character ID %q", i+1, cid),
				})
			}
		}

		if sc.EndSec > prevEnd {
			prevEnd = sc.EndSec
		}
	}

	return report
}
