package subtitles

import (
	"errors"
	"math"
	"strings"

	"reel/internal/ai/asr"
)

const cuePauseSplitSec = 0.55

// GenerateTrack converts a word-timestamped ASR transcript into styled, synchronized subtitle cues.
func GenerateTrack(transcript *asr.TranscribeResponse, style StyleConfig) (*Track, error) {
	if transcript == nil || len(transcript.Segments) == 0 {
		return nil, errors.New("cannot generate subtitles from empty transcript")
	}

	if style.MaxWordsPerLine <= 0 {
		style.MaxWordsPerLine = 4
	}
	if style.Template == "" {
		style.Template = "pop"
	}

	var cues []Cue
	var curWords []WordCue
	var sb strings.Builder

	flush := func() {
		if len(curWords) == 0 {
			return
		}
		cues = append(cues, Cue{
			Index: len(cues) + 1,
			Text:  strings.TrimSpace(sb.String()),
			Start: curWords[0].Start,
			End:   curWords[len(curWords)-1].End,
			Words: curWords,
		})
		curWords = nil
		sb.Reset()
	}

	for _, seg := range transcript.Segments {
		if len(seg.Words) == 0 {
			// Segment-only fallback when word timestamps are unavailable.
			txt := strings.TrimSpace(seg.Text)
			if txt != "" {
				durCS := int(math.Round((seg.End - seg.Start) * 100))
				if durCS < 10 {
					durCS = 10
				}
				cues = append(cues, Cue{
					Index: len(cues) + 1,
					Text:  txt,
					Start: seg.Start,
					End:   seg.End,
					Words: []WordCue{{Text: txt, Start: seg.Start, End: seg.End, DurationCS: durCS}},
				})
			}
			continue
		}

		for _, w := range seg.Words {
			txt := strings.TrimSpace(w.Text)
			if txt == "" {
				continue
			}

			if len(curWords) > 0 {
				prev := curWords[len(curWords)-1]
				if w.Start-prev.End >= cuePauseSplitSec || len(curWords) >= style.MaxWordsPerLine {
					flush()
				}
			}

			durCS := int(math.Round((w.End - w.Start) * 100))
			if durCS < 5 {
				durCS = 5
			}

			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(txt)
			curWords = append(curWords, WordCue{
				Text:       txt,
				Start:      w.Start,
				End:        w.End,
				DurationCS: durCS,
			})

			if endsClause(txt) {
				flush()
			}
		}
		flush()
	}

	return &Track{
		Style: style,
		Cues:  cues,
	}, nil
}

func endsClause(word string) bool {
	trimmed := strings.TrimRight(word, "\"')]} ")
	if trimmed == "" {
		return false
	}
	last := trimmed[len(trimmed)-1]
	return last == '.' || last == '!' || last == '?'
}
