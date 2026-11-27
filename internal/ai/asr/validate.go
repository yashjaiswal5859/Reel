package asr

import (
	"fmt"
	"strings"
)

// QualityIssue describes a potential defect found in a transcript (REQ-060).
type QualityIssue struct {
	Code         string  `json:"code"`
	Severity     string  `json:"severity"` // "error" | "warning"
	SegmentIndex int     `json:"segment_index"`
	WordIndex    int     `json:"word_index,omitempty"`
	Timestamp    float64 `json:"timestamp,omitempty"`
	Message      string  `json:"message"`
}

// QualityReport summarizes transcript validation results.
type QualityReport struct {
	Valid  bool           `json:"valid"`
	Issues []QualityIssue `json:"issues,omitempty"`
}

// ValidateTranscript checks completeness, timestamp monotonicity, word durations, and confidence.
func ValidateTranscript(resp *TranscribeResponse) QualityReport {
	report := QualityReport{Valid: true}
	if resp == nil || len(resp.Segments) == 0 {
		report.Valid = false
		report.Issues = append(report.Issues, QualityIssue{
			Code:     "EMPTY_TRANSCRIPT",
			Severity: "error",
			Message:  "transcript contains no segments",
		})
		return report
	}

	var prevSegEnd float64
	for si, seg := range resp.Segments {
		if strings.TrimSpace(seg.Text) == "" {
			report.Valid = false
			report.Issues = append(report.Issues, QualityIssue{
				Code:         "EMPTY_SEGMENT",
				Severity:     "error",
				SegmentIndex: si,
				Message:      fmt.Sprintf("segment %d has empty text", si),
			})
		}

		if seg.Start < prevSegEnd-0.001 {
			report.Valid = false
			report.Issues = append(report.Issues, QualityIssue{
				Code:         "SEGMENT_OVERLAP",
				Severity:     "error",
				SegmentIndex: si,
				Timestamp:    seg.Start,
				Message:      fmt.Sprintf("segment %d starts at %.2fs before previous end %.2fs", si, seg.Start, prevSegEnd),
			})
		}

		if si > 0 && seg.Start-prevSegEnd > 5.0 {
			report.Issues = append(report.Issues, QualityIssue{
				Code:         "SPEECH_GAP",
				Severity:     "warning",
				SegmentIndex: si,
				Timestamp:    prevSegEnd,
				Message:      fmt.Sprintf("large %.2fs gap before segment %d", seg.Start-prevSegEnd, si),
			})
		}

		var prevWordEnd float64
		for wi, w := range seg.Words {
			dur := w.End - w.Start
			if dur < 0 {
				report.Valid = false
				report.Issues = append(report.Issues, QualityIssue{
					Code:         "NEGATIVE_WORD_DURATION",
					Severity:     "error",
					SegmentIndex: si,
					WordIndex:    wi,
					Timestamp:    w.Start,
					Message:      fmt.Sprintf("word %q has negative duration", w.Text),
				})
			} else if dur < 0.02 || dur > 5.0 {
				report.Issues = append(report.Issues, QualityIssue{
					Code:         "SUSPICIOUS_WORD_DURATION",
					Severity:     "warning",
					SegmentIndex: si,
					WordIndex:    wi,
					Timestamp:    w.Start,
					Message:      fmt.Sprintf("word %q has unusual duration %.3fs", w.Text, dur),
				})
			}

			if w.Start < prevWordEnd-0.001 {
				report.Valid = false
				report.Issues = append(report.Issues, QualityIssue{
					Code:         "WORD_OVERLAP",
					Severity:     "error",
					SegmentIndex: si,
					WordIndex:    wi,
					Timestamp:    w.Start,
					Message:      fmt.Sprintf("word %q overlaps previous word", w.Text),
				})
			}

			if w.Confidence > 0 && w.Confidence < 0.5 {
				report.Issues = append(report.Issues, QualityIssue{
					Code:         "LOW_CONFIDENCE",
					Severity:     "warning",
					SegmentIndex: si,
					WordIndex:    wi,
					Timestamp:    w.Start,
					Message:      fmt.Sprintf("word %q has low confidence %.2f", w.Text, w.Confidence),
				})
			}
			prevWordEnd = w.End
		}

		if seg.End > prevSegEnd {
			prevSegEnd = seg.End
		}
	}

	return report
}
