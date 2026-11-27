package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"reel/internal/media"
	"reel/internal/temporal/activities"
)

const (
	QueryJobStatus = "status"
)

// VideoJobInput starts a stateless upload-to-result video processing workflow.
type VideoJobInput struct {
	JobID    string `json:"job_id"`
	InputKey string `json:"input_key"`
}

// VideoProcessingWorkflow orchestrates video preprocessing (and future AI stages).
func VideoProcessingWorkflow(ctx workflow.Context, input VideoJobInput) (*media.JobStatus, error) {
	status := media.JobStatus{
		JobID:    input.JobID,
		Stage:    media.StagePreprocessing,
		Progress: 10,
		InputKey: input.InputKey,
	}

	if err := workflow.SetQueryHandler(ctx, QueryJobStatus, func() (media.JobStatus, error) {
		return status, nil
	}); err != nil {
		return nil, err
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        2 * time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        30 * time.Second,
			MaximumAttempts:        3,
			NonRetryableErrorTypes: []string{"InvalidMediaError", "ConfigError"},
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	prepIn := activities.PreprocessInput{
		JobID:    input.JobID,
		InputKey: input.InputKey,
	}

	var meta media.VideoMetadata
	var prepActs *activities.PreprocessActivities
	if err := workflow.ExecuteActivity(actCtx, prepActs.InspectVideo, prepIn).Get(ctx, &meta); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}
	status.Metadata = &meta
	status.Progress = 35

	var prepOut activities.PreprocessOutput
	if err := workflow.ExecuteActivity(actCtx, prepActs.ExtractAudioAndPreview, prepIn).Get(ctx, &prepOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.AudioKey = prepOut.AudioKey
	status.PreviewKey = prepOut.PreviewKey
	status.Stage = media.StageTranscribing
	status.Progress = 65

	var asrOut activities.TranscribeOutput
	var asrActs *activities.ASRActivities
	if err := workflow.ExecuteActivity(actCtx, asrActs.TranscribeAudio, activities.TranscribeInput{
		JobID:    input.JobID,
		AudioKey: prepOut.AudioKey,
	}).Get(ctx, &asrOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.TranscriptKey = asrOut.TranscriptKey
	status.Stage = media.StageGeneratingSubtitles
	status.Progress = 80

	var subOut activities.SubtitleOutput
	var subActs *activities.SubtitleActivities
	if err := workflow.ExecuteActivity(actCtx, subActs.GenerateSubtitles, activities.SubtitleInput{
		JobID:         input.JobID,
		TranscriptKey: asrOut.TranscriptKey,
		Metadata:      &meta,
	}).Get(ctx, &subOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.SubtitlesKey = subOut.SubtitlesKey
	status.SubtitlesASSKey = subOut.SubtitlesASSKey
	status.Stage = media.StageGeneratingStoryboard
	status.Progress = 90

	var storyOut activities.StoryOutput
	var storyActs *activities.StoryActivities
	if err := workflow.ExecuteActivity(actCtx, storyActs.GenerateStoryboard, activities.StoryInput{
		JobID:            input.JobID,
		TranscriptKey:    asrOut.TranscriptKey,
		VideoDurationSec: meta.DurationSec,
	}).Get(ctx, &storyOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.StoryboardKey = storyOut.StoryboardKey
	status.Stage = media.StageGeneratingImages
	status.Progress = 92

	imgIn := activities.ImageGenInput{
		JobID:         input.JobID,
		StoryboardKey: storyOut.StoryboardKey,
		Width:         1024,
		Height:        1024,
	}
	var imgActs *activities.ImageActivities

	var charImgOut activities.ImageGenOutput
	if err := workflow.ExecuteActivity(actCtx, imgActs.GenerateCharacterRefs, imgIn).Get(ctx, &charImgOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.Progress = 90
	var sceneImgOut activities.ImageGenOutput
	if err := workflow.ExecuteActivity(actCtx, imgActs.GenerateSceneImages, imgIn).Get(ctx, &sceneImgOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.Stage = media.StageAnimating
	status.Progress = 95

	var animOut activities.AnimationOutput
	var animActs *activities.AnimationActivities
	if err := workflow.ExecuteActivity(actCtx, animActs.RenderAnimation, activities.AnimationInput{
		JobID:         input.JobID,
		StoryboardKey: storyOut.StoryboardKey,
		Width:         1080,
		Height:        1080,
		FPS:           30,
	}).Get(ctx, &animOut); err != nil {
		status.Stage = media.StageFailed
		status.Error = err.Error()
		return &status, err
	}

	status.AnimationKey = animOut.AnimationVideoKey
	status.Stage = media.StageAnimationReady
	status.Progress = 100

	return &status, nil
}
