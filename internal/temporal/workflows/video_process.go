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
	status.Stage = media.StageTranscribed
	status.Progress = 100

	return &status, nil
}
