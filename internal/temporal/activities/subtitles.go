package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"reel/internal/media"
	"reel/internal/storage"
	"reel/internal/subtitles"
)

// SubtitleActivities implements Temporal activities for generating subtitle tracks and ASS files.
type SubtitleActivities struct {
	Storage storage.Strategy
}

// SubtitleInput specifies the job, transcript key, video metadata, and optional style overrides.
type SubtitleInput struct {
	JobID         string                 `json:"job_id"`
	TranscriptKey string                 `json:"transcript_key"`
	Metadata      *media.VideoMetadata   `json:"metadata,omitempty"`
	Style         *subtitles.StyleConfig `json:"style,omitempty"`
}

// SubtitleOutput returns the stored subtitle JSON and ASS keys.
type SubtitleOutput struct {
	SubtitlesKey    string `json:"subtitles_key"`
	SubtitlesASSKey string `json:"subtitles_ass_key"`
	CueCount        int    `json:"cue_count"`
}

// GenerateSubtitles builds synchronized karaoke/animated subtitles from transcript.json.
func (a *SubtitleActivities) GenerateSubtitles(ctx context.Context, input SubtitleInput) (*SubtitleOutput, error) {
	rc, err := a.Storage.Get(ctx, input.TranscriptKey)
	if err != nil {
		return nil, fmt.Errorf("get transcript %s: %w", input.TranscriptKey, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}

	var doc StoredTranscript
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode transcript: %w", err)
	}

	style := subtitles.DefaultStyle()
	if input.Style != nil {
		style = *input.Style
	}

	track, err := subtitles.GenerateTrack(doc.Transcript, style)
	if err != nil {
		return nil, fmt.Errorf("generate subtitle track: %w", err)
	}

	jsonBytes, err := json.Marshal(track)
	if err != nil {
		return nil, fmt.Errorf("marshal subtitles: %w", err)
	}
	subKey := fmt.Sprintf("jobs/%s/subtitles.json", input.JobID)
	if err := a.Storage.Put(ctx, subKey, bytes.NewReader(jsonBytes), int64(len(jsonBytes)), "application/json"); err != nil {
		return nil, fmt.Errorf("store subtitles json: %w", err)
	}

	var assBuf bytes.Buffer
	if err := subtitles.RenderASS(&assBuf, track, input.Metadata); err != nil {
		return nil, fmt.Errorf("render ass: %w", err)
	}
	assKey := fmt.Sprintf("jobs/%s/subtitles.ass", input.JobID)
	if err := a.Storage.Put(ctx, assKey, bytes.NewReader(assBuf.Bytes()), int64(assBuf.Len()), "text/plain"); err != nil {
		return nil, fmt.Errorf("store subtitles ass: %w", err)
	}

	return &SubtitleOutput{
		SubtitlesKey:    subKey,
		SubtitlesASSKey: assKey,
		CueCount:        len(track.Cues),
	}, nil
}
