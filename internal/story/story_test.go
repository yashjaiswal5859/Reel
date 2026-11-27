package story_test

import (
	"context"
	"testing"

	"reel/internal/ai/asr"
	"reel/internal/ai/llm"
	"reel/internal/story"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequenceLLM struct {
	calls int
}

func (s *sequenceLLM) Name() string { return "mock-llm" }
func (s *sequenceLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	s.calls++
	if s.calls == 1 {
		return &llm.GenerateResponse{
			Provider: "mock-llm",
			Model:    "llama-3.3-70b",
			Usage:    llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			Content: "```json\n" + `{
				"title": "The Boy and the Sword",
				"summary": "A boy finds a glowing sword in a forest.",
				"tone": "adventurous",
				"characters": [
					{
						"id": "char_1",
						"name": "Leo",
						"physical_appearance": "10-year-old boy",
						"face_and_hairstyle": "messy brown hair",
						"clothing": "blue tunic",
						"body_proportions": "stylized 2D",
						"colors": ["blue", "brown"],
						"distinguishing_features": "scarf"
					}
				],
				"locations": ["mystic forest"],
				"key_objects": ["glowing sword"],
				"chronological_events": ["Leo picks up sword", "Leo walks into forest"],
				"relationships": []
			}` + "\n```",
		}, nil
	}
	return &llm.GenerateResponse{
		Provider: "mock-llm",
		Model:    "llama-3.3-70b",
		Usage:    llm.Usage{PromptTokens: 150, CompletionTokens: 80, TotalTokens: 230},
		Content: `{
			"scenes": [
				{
					"scene_id": "sc_1",
					"index": 1,
					"start_sec": 0.0,
					"end_sec": 4.0,
					"transcript_text": "Leo picked up the glowing sword.",
					"visual_type": "character_animation",
					"description": "Leo bends down and picks up a glowing sword from mossy stones.",
					"location": "mystic forest",
					"character_ids": ["char_1"],
					"objects": ["glowing sword"],
					"actions": ["PICK_UP"],
					"camera_motion": "zoom_in",
					"transition": "fade",
					"continuity_state": {"object_ownership": {"glowing sword": "char_1"}}
				},
				{
					"scene_id": "sc_2",
					"index": 2,
					"start_sec": 4.0,
					"end_sec": 8.0,
					"transcript_text": "Then he walked deeper into the woods.",
					"visual_type": "character_animation",
					"description": "Leo walks between tall trees holding the glowing sword.",
					"location": "mystic forest",
					"character_ids": ["char_1"],
					"objects": ["glowing sword"],
					"actions": ["WALK"],
					"camera_motion": "pan_right",
					"transition": "crossfade",
					"continuity_state": {"object_ownership": {"glowing sword": "char_1"}}
				}
			]
		}`,
	}, nil
}

func TestDirector_DirectAndValidate(t *testing.T) {
	dir, err := story.NewDirector(&sequenceLLM{})
	require.NoError(t, err)

	transcript := &asr.TranscribeResponse{
		Segments: []asr.Segment{
			{Text: "Leo picked up the glowing sword.", Start: 0.0, End: 4.0},
			{Text: "Then he walked deeper into the woods.", Start: 4.0, End: 8.0},
		},
	}

	res, err := dir.Direct(context.Background(), transcript, story.Style2DCartoon)
	require.NoError(t, err)
	assert.Equal(t, 380, res.Usage.TotalTokens)
	require.Len(t, res.Storyboard.CharacterBible, 1)
	require.Len(t, res.Storyboard.Scenes, 2)
	assert.Equal(t, "char_1", res.Storyboard.Scenes[1].Continuity.ObjectOwnership["glowing sword"])

	report := story.ValidateStoryboard(res.Storyboard, 10.0)
	assert.True(t, report.Valid)
	assert.Empty(t, report.Issues)
}

func TestValidateStoryboard_CatchesOverlapAndUnknownChar(t *testing.T) {
	sb := &story.Storyboard{
		CharacterBible: []story.CharacterProfile{{ID: "char_1", Name: "Leo"}},
		Scenes: []story.Scene{
			{Index: 1, StartSec: 0, EndSec: 5, Description: "Scene 1", VisualType: story.VisualCharacterAnimation, CharacterIDs: []string{"char_1"}},
			{Index: 2, StartSec: 4, EndSec: 8, Description: "Scene 2", VisualType: story.VisualCharacterAnimation, CharacterIDs: []string{"char_missing"}},
		},
	}

	report := story.ValidateStoryboard(sb, 10.0)
	assert.False(t, report.Valid)
	assert.GreaterOrEqual(t, len(report.Issues), 2)
}
