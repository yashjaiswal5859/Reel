package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"go.temporal.io/sdk/temporal"
	"reel/internal/ai/llm"
	"reel/internal/storage"
	"reel/internal/story"
)

// StoryActivities implements Temporal activities for AI story understanding and storyboard generation.
type StoryActivities struct {
	Storage storage.Strategy
	LLM     llm.Strategy
}

// StoryInput specifies the job, transcript location, and desired visual style.
type StoryInput struct {
	JobID            string            `json:"job_id"`
	TranscriptKey    string            `json:"transcript_key"`
	VideoDurationSec float64           `json:"video_duration_sec"`
	Style            story.VisualStyle `json:"style,omitempty"`
}

// StoryOutput returns the stored storyboard key and validation summary.
type StoryOutput struct {
	StoryboardKey string              `json:"storyboard_key"`
	SceneCount    int                 `json:"scene_count"`
	Quality       story.QualityReport `json:"quality"`
	Usage         llm.Usage           `json:"usage"`
}

// StoredStoryboard is the JSON artifact saved to temporary storage.
type StoredStoryboard struct {
	Storyboard *story.Storyboard   `json:"storyboard"`
	Quality    story.QualityReport `json:"quality"`
	Provider   string              `json:"provider"`
	Model      string              `json:"model"`
	Usage      llm.Usage           `json:"usage"`
}

// GenerateStoryboard reads transcript.json, directs the storyboard, validates it, and stores storyboard.json.
func (a *StoryActivities) GenerateStoryboard(ctx context.Context, input StoryInput) (*StoryOutput, error) {
	if a.LLM == nil {
		return nil, temporal.NewNonRetryableApplicationError("llm provider not configured", "ConfigError", nil)
	}

	rc, err := a.Storage.Get(ctx, input.TranscriptKey)
	if err != nil {
		return nil, fmt.Errorf("get transcript %s: %w", input.TranscriptKey, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}

	var tdoc StoredTranscript
	if err := json.Unmarshal(raw, &tdoc); err != nil {
		return nil, fmt.Errorf("decode transcript: %w", err)
	}

	director, err := story.NewDirector(a.LLM)
	if err != nil {
		return nil, err
	}

	res, err := director.Direct(ctx, tdoc.Transcript, input.Style)
	if err != nil {
		return nil, fmt.Errorf("direct storyboard: %w", err)
	}

	quality := story.ValidateStoryboard(res.Storyboard, input.VideoDurationSec)
	doc := StoredStoryboard{
		Storyboard: res.Storyboard,
		Quality:    quality,
		Provider:   res.Provider,
		Model:      res.Model,
		Usage:      res.Usage,
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal storyboard: %w", err)
	}

	sbKey := fmt.Sprintf("jobs/%s/storyboard.json", input.JobID)
	if err := a.Storage.Put(ctx, sbKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		return nil, fmt.Errorf("store storyboard: %w", err)
	}

	return &StoryOutput{
		StoryboardKey: sbKey,
		SceneCount:    len(res.Storyboard.Scenes),
		Quality:       quality,
		Usage:         res.Usage,
	}, nil
}
