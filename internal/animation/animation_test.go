package animation_test

import (
	"testing"

	"reel/internal/animation"
	"reel/internal/story"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanTimeline_MapsActionsCameraAndEnvironment(t *testing.T) {
	sb := &story.Storyboard{
		Style: story.Style2DCartoon,
		Scenes: []story.Scene{
			{
				SceneID:      "sc_1",
				Index:        1,
				StartSec:     0.0,
				EndSec:       4.0,
				Location:     "rainy forest",
				Description:  "Leo runs through the rain",
				CharacterIDs: []string{"char_1"},
				Objects:      []string{"sword"},
				Actions:      []string{"RUN"},
				CameraMotion: "pan_right",
				Transition:   "slide_left",
			},
			{
				SceneID:      "sc_2",
				Index:        2,
				StartSec:     4.0,
				EndSec:       7.0,
				Location:     "warm campfire",
				Description:  "Fire glows near the tent",
				CameraMotion: "zoom_in",
				Transition:   "crossfade",
			},
		},
	}

	tl, err := animation.PlanTimeline("job-1", sb, 1080, 1080, 30)
	require.NoError(t, err)
	assert.Equal(t, 7.0, tl.TotalDurationSec)
	require.Len(t, tl.Scenes, 2)

	assert.Equal(t, animation.CamPanRight, tl.Scenes[0].Camera)
	assert.Equal(t, animation.EnvRain, tl.Scenes[0].Environment)
	assert.Equal(t, animation.TransSlideLeft, tl.Scenes[0].Transition)
	assert.GreaterOrEqual(t, len(tl.Scenes[0].Events), 3)

	assert.Equal(t, animation.CamZoomIn, tl.Scenes[1].Camera)
	assert.Equal(t, animation.EnvFireGlow, tl.Scenes[1].Environment)
}

func TestBuildSceneFilter_AndRendererFactory(t *testing.T) {
	r, err := animation.NewRenderer("ffmpeg", "ffmpeg")
	require.NoError(t, err)
	assert.Equal(t, "ffmpeg", r.Name())

	_, err = animation.NewRenderer("unknown-renderer", "")
	require.Error(t, err)

	f := animation.BuildSceneFilter(animation.ScenePlan{
		DurationSec: 3.0,
		Camera:      animation.CamShake,
		Environment: animation.EnvFireGlow,
	}, 1080, 1080, 30)
	assert.Contains(t, f, "zoompan=")
	assert.Contains(t, f, "sin(on*1.2)")
	assert.Contains(t, f, "eq=brightness=")

	assert.Equal(t, "slideleft", animation.MapXfadeTransition(animation.TransSlideLeft))
	assert.Equal(t, "wipeleft", animation.MapXfadeTransition(animation.TransWipe))
}
