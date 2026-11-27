package story_test

import (
	"testing"

	"reel/internal/story"

	"github.com/stretchr/testify/assert"
)

func TestBuildPrompts_IncludesStyleBibleAndContinuity(t *testing.T) {
	char := story.CharacterProfile{
		ID:                 "char_1",
		Name:               "Leo",
		PhysicalAppearance: "10-year-old boy",
		FaceAndHairstyle:   "messy brown hair",
		Clothing:           "blue tunic",
		Colors:             []string{"blue", "gold"},
	}

	cp, neg := story.BuildCharacterSheetPrompt(char, story.StyleAnime)
	assert.Contains(t, cp, "anime")
	assert.Contains(t, cp, "Leo")
	assert.Contains(t, cp, "blue tunic")
	assert.NotEmpty(t, neg)

	scene := story.Scene{
		SceneID:      "sc_2",
		Description:  "Leo walks through a glowing forest",
		Location:     "mystic forest",
		CharacterIDs: []string{"char_1"},
		Objects:      []string{"glowing sword"},
		Actions:      []string{"WALK"},
		CameraMotion: "pan_right",
		Continuity: story.NarrativeState{
			ObjectOwnership: map[string]string{"glowing sword": "char_1"},
		},
	}

	sp, _ := story.BuildScenePrompt(scene, []story.CharacterProfile{char}, story.StyleAnime)
	assert.Contains(t, sp, "anime")
	assert.Contains(t, sp, "Leo (10-year-old boy")
	assert.Contains(t, sp, "Leo holding glowing sword")
	assert.Contains(t, sp, "mystic forest")
}
