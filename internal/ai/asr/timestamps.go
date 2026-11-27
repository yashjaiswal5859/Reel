package asr

import "strings"

const (
	pauseThresholdSec = 0.85
	maxWordsPerSeg    = 14
)

// NormalizeWordTimestamps ensures non-negative, monotonic, non-overlapping word timings.
func NormalizeWordTimestamps(words []Word) {
	var prevEnd float64
	for i := range words {
		if words[i].Start < 0 {
			words[i].Start = 0
		}
		if words[i].Start < prevEnd {
			words[i].Start = prevEnd
		}
		if words[i].End < words[i].Start {
			words[i].End = words[i].Start + 0.05
		}
		prevEnd = words[i].End
	}
}

// GroupWordsIntoSentences groups word-level timestamps into readable sentences
// based on punctuation, speech pauses, and a maximum word cap.
func GroupWordsIntoSentences(words []Word) []Segment {
	if len(words) == 0 {
		return nil
	}

	var segments []Segment
	var curWords []Word
	var sb strings.Builder

	flush := func() {
		if len(curWords) == 0 {
			return
		}
		segments = append(segments, Segment{
			Text:  strings.TrimSpace(sb.String()),
			Start: curWords[0].Start,
			End:   curWords[len(curWords)-1].End,
			Words: curWords,
		})
		curWords = nil
		sb.Reset()
	}

	for i, w := range words {
		if len(curWords) > 0 {
			prev := curWords[len(curWords)-1]
			if w.Start-prev.End >= pauseThresholdSec || len(curWords) >= maxWordsPerSeg {
				flush()
			}
		}

		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(w.Text)
		curWords = append(curWords, w)

		if endsSentence(w.Text) || i == len(words)-1 {
			flush()
		}
	}

	return segments
}

// RebuildTranscriptFromSegments recalculates full text and segment boundaries after user edits.
func RebuildTranscriptFromSegments(resp *TranscribeResponse) {
	if resp == nil {
		return
	}
	var full strings.Builder
	for i := range resp.Segments {
		seg := &resp.Segments[i]
		if len(seg.Words) > 0 {
			NormalizeWordTimestamps(seg.Words)
			var segText strings.Builder
			for j, w := range seg.Words {
				if j > 0 {
					segText.WriteByte(' ')
				}
				segText.WriteString(strings.TrimSpace(w.Text))
			}
			seg.Text = segText.String()
			seg.Start = seg.Words[0].Start
			seg.End = seg.Words[len(seg.Words)-1].End
		}
		if strings.TrimSpace(seg.Text) != "" {
			if full.Len() > 0 {
				full.WriteByte(' ')
			}
			full.WriteString(strings.TrimSpace(seg.Text))
		}
	}
	resp.Text = full.String()
}

func endsSentence(word string) bool {
	trimmed := strings.TrimRight(word, "\"')]} ")
	if trimmed == "" {
		return false
	}
	last := trimmed[len(trimmed)-1]
	return last == '.' || last == '!' || last == '?'
}
