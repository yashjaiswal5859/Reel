package workflows_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"reel/internal/ai/asr"
	"reel/internal/ai/imagegen"
	"reel/internal/ai/llm"
	"reel/internal/animation"
	"reel/internal/media"
	"reel/internal/storage"
	"reel/internal/temporal/activities"
	"reel/internal/temporal/workflows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

type stubMediaProcessor struct {
	meta *media.VideoMetadata
	err  error
}

func (s *stubMediaProcessor) Probe(_ context.Context, _ string) (*media.VideoMetadata, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.meta, nil
}

func (s *stubMediaProcessor) ExtractNormalizedAudio(_ context.Context, _, outputPath string) error {
	return os.WriteFile(outputPath, []byte("RIFF-wav-bytes"), 0600)
}

func (s *stubMediaProcessor) GeneratePreview(_ context.Context, _, outputPath string) error {
	return os.WriteFile(outputPath, []byte("mp4-preview-bytes"), 0600)
}

func (s *stubMediaProcessor) BurnSubtitles(_ context.Context, _, _, outputPath string) error {
	return os.WriteFile(outputPath, []byte("mp4-burned-bytes"), 0600)
}

type stubASRStrategy struct{}

func (s *stubASRStrategy) Name() string { return "stub-asr" }
func (s *stubASRStrategy) Transcribe(_ context.Context, _ asr.TranscribeRequest) (*asr.TranscribeResponse, error) {
	return &asr.TranscribeResponse{
		Provider: "stub-asr",
		Language: "en",
		Text:     "Hello story.",
		Segments: []asr.Segment{
			{
				Text:  "Hello story.",
				Start: 0.0,
				End:   1.0,
				Words: []asr.Word{
					{Text: "Hello", Start: 0.0, End: 0.4},
					{Text: "story.", Start: 0.4, End: 1.0},
				},
			},
		},
	}, nil
}

type stubLLMStrategy struct {
	calls int
}

func (s *stubLLMStrategy) Name() string { return "stub-llm" }
func (s *stubLLMStrategy) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, error) {
	s.calls++
	if s.calls == 1 {
		return &llm.GenerateResponse{
			Provider: "stub-llm",
			Content:  `{"title":"Test","summary":"Test","tone":"happy","characters":[{"id":"char_1","name":"Hero","physical_appearance":"boy"}],"locations":["park"],"key_objects":[],"chronological_events":["walks"],"relationships":[]}`,
		}, nil
	}
	return &llm.GenerateResponse{
		Provider: "stub-llm",
		Content:  `{"scenes":[{"scene_id":"sc_1","index":1,"start_sec":0,"end_sec":1,"transcript_text":"Hello story.","visual_type":"character_animation","description":"Hero waves in park","location":"park","character_ids":["char_1"],"objects":[],"actions":["WAVE"],"camera_motion":"zoom_in","transition":"fade","continuity_state":{}}]}`,
	}, nil
}

type stubImageGenStrategy struct{}

func (s *stubImageGenStrategy) Name() string { return "stub-img" }
func (s *stubImageGenStrategy) Generate(_ context.Context, _ imagegen.GenerateRequest) (*imagegen.GenerateResponse, error) {
	return &imagegen.GenerateResponse{
		Provider:    "stub-img",
		ImageBytes:  []byte("PNG-IMAGE-BYTES"),
		ContentType: "image/png",
	}, nil
}

type stubAnimRenderer struct{}

func (s *stubAnimRenderer) Name() string { return "stub-anim" }
func (s *stubAnimRenderer) RenderTimeline(_ context.Context, _ *animation.Timeline, _ map[string]string, outputPath string) error {
	return os.WriteFile(outputPath, []byte("MP4-ANIM-BYTES"), 0600)
}

func TestVideoProcessingWorkflow_Success(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	inputKey := "jobs/job-123/input.mp4"
	require.NoError(t, st.Put(ctx, inputKey, bytes.NewReader([]byte("fake-mp4")), 8, "video/mp4"))

	prepActs := &activities.PreprocessActivities{
		Storage: st,
		Media: &stubMediaProcessor{
			meta: &media.VideoMetadata{
				DurationSec: 25.0,
				Width:       1080,
				Height:      1920,
				FPS:         30,
				HasAudio:    true,
				Orientation: "portrait",
			},
		},
		MaxDurationSec: 600,
	}

	asrActs := &activities.ASRActivities{
		Storage: st,
		ASR:     &stubASRStrategy{},
	}
	subActs := &activities.SubtitleActivities{
		Storage: st,
	}
	storyActs := &activities.StoryActivities{
		Storage: st,
		LLM:     &stubLLMStrategy{},
	}
	imgActs := &activities.ImageActivities{
		Storage:        st,
		ImageGen:       &stubImageGenStrategy{},
		MaxConcurrency: 2,
	}
	animActs := &activities.AnimationActivities{
		Storage:  st,
		Renderer: &stubAnimRenderer{},
	}

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(prepActs)
	env.RegisterActivity(asrActs)
	env.RegisterActivity(subActs)
	env.RegisterActivity(storyActs)
	env.RegisterActivity(imgActs)
	env.RegisterActivity(animActs)

	env.ExecuteWorkflow(workflows.VideoProcessingWorkflow, workflows.VideoJobInput{
		JobID:    "job-123",
		InputKey: inputKey,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result media.JobStatus
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, media.StageAnimationReady, result.Stage)
	assert.Equal(t, 100, result.Progress)
	assert.Equal(t, "jobs/job-123/audio.wav", result.AudioKey)
	assert.Equal(t, "jobs/job-123/preview.mp4", result.PreviewKey)
	assert.Equal(t, "jobs/job-123/transcript.json", result.TranscriptKey)
	assert.Equal(t, "jobs/job-123/subtitles.json", result.SubtitlesKey)
	assert.Equal(t, "jobs/job-123/subtitles.ass", result.SubtitlesASSKey)
	assert.Equal(t, "jobs/job-123/storyboard.json", result.StoryboardKey)
	assert.Equal(t, "jobs/job-123/animation.mp4", result.AnimationKey)
	require.NotNil(t, result.Metadata)
	assert.Equal(t, "portrait", result.Metadata.Orientation)

	// Verify all artifacts actually landed in storage
	for _, k := range []string{
		result.AudioKey,
		result.PreviewKey,
		result.TranscriptKey,
		result.SubtitlesKey,
		result.SubtitlesASSKey,
		result.StoryboardKey,
		result.AnimationKey,
		"jobs/job-123/characters/char_1.png",
		"jobs/job-123/scenes/sc_1.png",
		"jobs/job-123/animation_timeline.json",
	} {
		exists, err := st.Exists(ctx, k)
		require.NoError(t, err)
		assert.True(t, exists, "expected %s in storage", k)
	}
}

func TestVideoProcessingWorkflow_RejectsOverDuration(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	inputKey := "jobs/job-too-long/input.mp4"
	require.NoError(t, st.Put(ctx, inputKey, bytes.NewReader([]byte("fake-mp4")), 8, "video/mp4"))

	prepActs := &activities.PreprocessActivities{
		Storage: st,
		Media: &stubMediaProcessor{
			meta: &media.VideoMetadata{
				DurationSec: 900, // exceeds 600s limit
				Width:       1920,
				Height:      1080,
				HasAudio:    true,
			},
		},
		MaxDurationSec: 600,
	}

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(prepActs)

	env.ExecuteWorkflow(workflows.VideoProcessingWorkflow, workflows.VideoJobInput{
		JobID:    "job-too-long",
		InputKey: inputKey,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "exceeds limit")
}
