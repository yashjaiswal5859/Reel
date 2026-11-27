package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/api"
	"reel/internal/cost"
	"reel/internal/logger"
	"reel/internal/media"
	"reel/internal/storage"
	"reel/internal/temporal/workflows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubStageJobService struct {
	status *media.JobStatus
}

func (s *stubStageJobService) StartVideoJob(_ context.Context, _ workflows.VideoJobInput) error {
	return nil
}

func (s *stubStageJobService) GetJobStatus(_ context.Context, _ string) (*media.JobStatus, error) {
	return s.status, nil
}

func (s *stubStageJobService) CancelJob(_ context.Context, _ string) error {
	return nil
}

func TestStages_Transparency(t *testing.T) {
	svc := &stubStageJobService{
		status: &media.JobStatus{
			JobID:         "job-stage-1",
			Stage:         media.StageGeneratingStoryboard,
			Progress:      85,
			InputKey:      "jobs/job-stage-1/input.mp4",
			AudioKey:      "jobs/job-stage-1/audio.wav",
			TranscriptKey: "jobs/job-stage-1/transcript.json",
			SubtitlesKey:  "jobs/job-stage-1/subtitles.json",
			Metadata:      &media.VideoMetadata{DurationSec: 45},
		},
	}

	r := api.NewRouter(api.RouterDeps{
		Logger: logger.New("error", "test"),
		Env:    "test",
		Jobs:   svc,
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-stage-1/stages", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		JobID    string            `json:"job_id"`
		Stage    media.JobStage    `json:"stage"`
		Progress int               `json:"progress"`
		Stages   []api.StageDetail `json:"stages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, media.StageGeneratingStoryboard, resp.Stage)
	require.Len(t, resp.Stages, 9)
	assert.Equal(t, "completed", resp.Stages[0].State)
	assert.Equal(t, "completed", resp.Stages[3].State)
	assert.Equal(t, "in_progress", resp.Stages[4].State)
	assert.Equal(t, "pending", resp.Stages[5].State)
}

func TestCost_EstimateAndJobCost(t *testing.T) {
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	seedStoryboard(t, st, "job-cost")

	r := api.NewRouter(api.RouterDeps{
		Logger:          logger.New("error", "test"),
		Env:             "test",
		Storage:         st,
		DefaultASR:      "groq",
		DefaultLLM:      "groq",
		DefaultImageGen: "replicate",
	})

	// Pre-upload estimate
	req := httptest.NewRequest(http.MethodPost, "/api/v1/estimate", bytes.NewBufferString(`{"duration_sec":30,"quality":"1080p"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var est cost.CostEstimate
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &est))
	assert.Equal(t, "groq", est.Assumptions.ASRProvider)
	assert.Equal(t, "replicate", est.Assumptions.ImageGenProvider)
	assert.Greater(t, est.EstimatedCredits, 0)

	// Invalid duration
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/estimate", bytes.NewBufferString(`{"duration_sec":0}`))
	badReq.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, badReq)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)

	// Job-specific cost from storyboard
	wJob := httptest.NewRecorder()
	r.ServeHTTP(wJob, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-cost/cost", nil))
	require.Equal(t, http.StatusOK, wJob.Code)

	var jobCost struct {
		JobID    string            `json:"job_id"`
		IsActual bool              `json:"is_actual"`
		Estimate cost.CostEstimate `json:"estimate"`
	}
	require.NoError(t, json.Unmarshal(wJob.Body.Bytes(), &jobCost))
	assert.True(t, jobCost.IsActual)
	assert.Equal(t, 1, jobCost.Estimate.Assumptions.SceneCount)
}
