package story

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"reel/internal/ai/asr"
	"reel/internal/ai/llm"
)

// Director uses an LLM Strategy to understand the full story and generate a continuous storyboard.
type Director struct {
	llm llm.Strategy
}

// NewDirector creates a Story Director backed by the given LLM Strategy.
func NewDirector(provider llm.Strategy) (*Director, error) {
	if provider == nil {
		return nil, errors.New("llm provider is required")
	}
	return &Director{llm: provider}, nil
}

// DirectResult bundles the generated storyboard and aggregate token usage.
type DirectResult struct {
	Storyboard *Storyboard `json:"storyboard"`
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	Usage      llm.Usage   `json:"usage"`
}

// Direct runs full-story analysis followed by continuity-aware storyboard generation.
func (d *Director) Direct(ctx context.Context, transcript *asr.TranscribeResponse, style VisualStyle) (*DirectResult, error) {
	if transcript == nil || len(transcript.Segments) == 0 {
		return nil, errors.New("cannot direct storyboard from empty transcript")
	}
	if style == "" {
		style = Style2DCartoon
	}

	formattedTranscript := formatTimedTranscript(transcript)

	analysis, u1, model1, err := d.analyzeStory(ctx, formattedTranscript)
	if err != nil {
		return nil, fmt.Errorf("story analysis: %w", err)
	}

	scenes, u2, model2, err := d.generateStoryboard(ctx, formattedTranscript, analysis, style)
	if err != nil {
		return nil, fmt.Errorf("storyboard generation: %w", err)
	}

	sb := &Storyboard{
		Style:          style,
		Analysis:       *analysis,
		CharacterBible: analysis.Characters,
		Scenes:         scenes,
	}

	usedModel := model2
	if usedModel == "" {
		usedModel = model1
	}

	return &DirectResult{
		Storyboard: sb,
		Provider:   d.llm.Name(),
		Model:      usedModel,
		Usage: llm.Usage{
			PromptTokens:     u1.PromptTokens + u2.PromptTokens,
			CompletionTokens: u1.CompletionTokens + u2.CompletionTokens,
			TotalTokens:      u1.TotalTokens + u2.TotalTokens,
		},
	}, nil
}

func (d *Director) analyzeStory(ctx context.Context, timedTranscript string) (*StoryAnalysis, llm.Usage, string, error) {
	sysPrompt := `You are an expert story analyst and animation director.
Read the ENTIRE timed transcript first to understand the complete story before breaking it down.
Return ONLY valid JSON matching this schema:
{
  "title": "string",
  "summary": "string",
  "tone": "string",
  "characters": [
    {
      "id": "char_1",
      "name": "string",
      "physical_appearance": "string",
      "face_and_hairstyle": "string",
      "clothing": "string",
      "body_proportions": "string",
      "colors": ["string"],
      "distinguishing_features": "string"
    }
  ],
  "locations": ["string"],
  "key_objects": ["string"],
  "chronological_events": ["string"],
  "relationships": ["string"]
}`

	resp, err := d.llm.Generate(ctx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: "Timed Transcript:\n" + timedTranscript},
		},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		return nil, llm.Usage{}, "", err
	}

	var analysis StoryAnalysis
	if err := json.Unmarshal([]byte(stripJSONFence(resp.Content)), &analysis); err != nil {
		return nil, resp.Usage, resp.Model, fmt.Errorf("parse story analysis json: %w", err)
	}
	return &analysis, resp.Usage, resp.Model, nil
}

func (d *Director) generateStoryboard(ctx context.Context, timedTranscript string, analysis *StoryAnalysis, style VisualStyle) ([]Scene, llm.Usage, string, error) {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return nil, llm.Usage{}, "", err
	}

	sysPrompt := fmt.Sprintf(`You are an AI Scene Director for a %s animated storytelling video.
Using the Story Analysis (Character Bible, locations, objects) and the Timed Transcript, divide the narration into visual scenes.
Rules:
1. Cover the transcript chronologically with no negative or overlapping timestamps.
2. Choose the best visual_type for each scene from: character_animation, environment_illustration, object_animation, animated_diagram, chart_infographic, kinetic_typography, visual_metaphor, static_motion.
3. Maintain narrative continuity in continuity_state (if a character picks up an object in scene 1, keep it in object_ownership in scene 2 unless put down).
4. Reference characters ONLY by their IDs from the Character Bible.
5. Return ONLY valid JSON: {"scenes": [ ... ]} where each scene has:
   scene_id, index (1-based), start_sec, end_sec, transcript_text, visual_type, description, location, character_ids, objects, actions, camera_motion, transition, continuity_state.`, style)

	userMsg := fmt.Sprintf("Story Analysis:\n%s\n\nTimed Transcript:\n%s", string(analysisJSON), timedTranscript)

	resp, err := d.llm.Generate(ctx, llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userMsg},
		},
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, llm.Usage{}, "", err
	}

	var wrapper struct {
		Scenes []Scene `json:"scenes"`
	}
	if err := json.Unmarshal([]byte(stripJSONFence(resp.Content)), &wrapper); err != nil {
		return nil, resp.Usage, resp.Model, fmt.Errorf("parse storyboard json: %w", err)
	}
	return wrapper.Scenes, resp.Usage, resp.Model, nil
}

func formatTimedTranscript(t *asr.TranscribeResponse) string {
	var sb strings.Builder
	for _, seg := range t.Segments {
		fmt.Fprintf(&sb, "[%.2fs - %.2fs] %s\n", seg.Start, seg.End, seg.Text)
	}
	return sb.String()
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}
