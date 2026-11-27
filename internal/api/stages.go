package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"reel/internal/media"
)

// StageDetail describes a single pipeline stage's progress and output artifact.
type StageDetail struct {
	Stage       media.JobStage `json:"stage"`
	Label       string         `json:"label"`
	State       string         `json:"state"` // "completed" | "in_progress" | "pending" | "failed" | "cancelled"
	ArtifactKey string         `json:"artifact_key,omitempty"`
}

type stageSpec struct {
	stage media.JobStage
	label string
	keyFn func(s *media.JobStatus) string
}

var pipelineStages = []stageSpec{
	{media.StageUploaded, "Video Uploaded", func(s *media.JobStatus) string { return s.InputKey }},
	{media.StagePreprocessing, "Audio Extraction & Inspection", func(s *media.JobStatus) string { return s.AudioKey }},
	{media.StageTranscribing, "Speech Recognition (ASR)", func(s *media.JobStatus) string { return s.TranscriptKey }},
	{media.StageGeneratingSubtitles, "Subtitle Generation", func(s *media.JobStatus) string { return s.SubtitlesKey }},
	{media.StageGeneratingStoryboard, "Story Understanding & Storyboard", func(s *media.JobStatus) string { return s.StoryboardKey }},
	{media.StageGeneratingImages, "Character & Scene Illustration", func(s *media.JobStatus) string { return s.StoryboardKey }},
	{media.StageAnimating, "Scene Animation", func(s *media.JobStatus) string { return s.AnimationKey }},
	{media.StageComposing, "Split-Screen Composition", func(s *media.JobStatus) string { return s.FinalVideoKey }},
	{media.StageCompleted, "Completed", func(s *media.JobStatus) string { return s.FinalVideoKey }},
}

// stageAliases maps intermediate "ready" stages to their primary stage index.
var stageOrder = map[media.JobStage]int{
	media.StageUploaded:             0,
	media.StagePreprocessing:        1,
	media.StageAudioExtracted:       1,
	media.StageTranscribing:         2,
	media.StageTranscribed:          2,
	media.StageGeneratingSubtitles:  3,
	media.StageSubtitlesReady:       3,
	media.StageGeneratingStoryboard: 4,
	media.StageStoryboardReady:      4,
	media.StageGeneratingImages:     5,
	media.StageImagesReady:          5,
	media.StageAnimating:            6,
	media.StageAnimationReady:       6,
	media.StageComposing:            7,
	media.StageCompleted:            8,
}

func (h *jobHandler) getStages(c *gin.Context) {
	if h.jobs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "job service not configured"})
		return
	}
	jobID := c.Param("id")
	status, err := h.jobs.GetJobStatus(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("job %q not found: %v", jobID, err)})
		return
	}

	details := buildStageDetails(status)
	c.JSON(http.StatusOK, gin.H{
		"job_id":   status.JobID,
		"stage":    status.Stage,
		"progress": status.Progress,
		"error":    status.Error,
		"stages":   details,
	})
}

func buildStageDetails(status *media.JobStatus) []StageDetail {
	currentIdx, known := stageOrder[status.Stage]
	if !known {
		currentIdx = -1
	}

	details := make([]StageDetail, len(pipelineStages))
	for i, spec := range pipelineStages {
		artKey := spec.keyFn(status)
		state := "pending"

		switch status.Stage {
		case media.StageCompleted:
			state = "completed"
		case media.StageFailed:
			if artKey != "" {
				state = "completed"
			} else {
				state = "failed"
			}
		case media.StageCancelled:
			if artKey != "" {
				state = "completed"
			} else {
				state = "cancelled"
			}
		default:
			if known {
				if i < currentIdx {
					state = "completed"
				} else if i == currentIdx {
					state = "in_progress"
				}
			}
		}

		details[i] = StageDetail{
			Stage:       spec.stage,
			Label:       spec.label,
			State:       state,
			ArtifactKey: artKey,
		}
	}
	return details
}
