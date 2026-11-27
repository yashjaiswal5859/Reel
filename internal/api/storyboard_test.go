package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/ai/imagegen"
	"reel/internal/api"
	"reel/internal/logger"
	"reel/internal/storage"
	"reel/internal/story"
	"reel/internal/temporal/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubImageGen struct {
	calls int
}

func (s *stubImageGen) Name() string { return "stub-imagegen" }

func (s *stubImageGen) Generate(_ context.Context, req imagegen.GenerateRequest) (*imagegen.GenerateResponse, error) {
	s.calls++
	return &imagegen.GenerateResponse{
		ImageBytes:  []byte("fake-png-bytes-" + req.Prompt),
		ContentType: "image/png",
		Provider:    "stub",
		Model:       "stub-v1",
	}, nil
}

func seedStoryboard(t *testing.T, st storage.Strategy, jobID string) {
	t.Helper()
	doc := activities.StoredStoryboard{
		Provider: "groq",
		Model:    "llama-3.3-70b-versatile",
		Quality:  story.QualityReport{Valid: true},
		Storyboard: &story.Storyboard{
			Style: story.Style2DCartoon,
			Analysis: story.StoryAnalysis{
				Title:   "Forest Tale",
				Summary: "A short story in a forest.",
			},
			Scenes: []story.Scene{
				{
					SceneID:     "sc_1",
					Index:       1,
					StartSec:    0,
					EndSec:      5,
					VisualType:  story.VisualEnvironment,
					Description: "Forest intro",
					Actions:     []string{"pan_in"},
					ImageKey:    "jobs/" + jobID + "/scenes/sc_1.png",
				},
			},
		},
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, st.Put(context.Background(), "jobs/"+jobID+"/storyboard.json", bytes.NewReader(raw), int64(len(raw)), "application/json"))
}

func TestStoryboard_Get(t *testing.T) {
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	seedStoryboard(t, st, "job-sb")

	r := api.NewRouter(api.RouterDeps{
		Logger:  logger.New("error", "test"),
		Env:     "test",
		Storage: st,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-sb/storyboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var got activities.StoredStoryboard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "groq", got.Provider)
	require.Len(t, got.Storyboard.Scenes, 1)

	missReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/missing/storyboard", nil)
	missW := httptest.NewRecorder()
	r.ServeHTTP(missW, missReq)
	assert.Equal(t, http.StatusNotFound, missW.Code)
}

func TestStoryboard_ScenesViewerAndPatch(t *testing.T) {
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	seedStoryboard(t, st, "job-edit")

	r := api.NewRouter(api.RouterDeps{
		Logger:  logger.New("error", "test"),
		Env:     "test",
		Storage: st,
	})

	// List scene cards
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-edit/storyboard/scenes", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var listResp struct {
		JobID  string          `json:"job_id"`
		Scenes []api.SceneCard `json:"scenes"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listResp))
	require.Len(t, listResp.Scenes, 1)
	assert.Equal(t, 5.0, listResp.Scenes[0].DurationSec)
	assert.Equal(t, "/api/v1/jobs/job-edit/assets/scenes/sc_1.png", listResp.Scenes[0].PreviewURL)

	// Get single scene
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-edit/storyboard/scenes/sc_1", nil))
	require.Equal(t, http.StatusOK, wGet.Code)

	// Patch scene description, duration_sec, actions
	patchBody := `{"description":"Updated moonlit forest","duration_sec":7.5,"actions":["zoom_in","leaves_fall"]}`
	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/jobs/job-edit/storyboard/scenes/sc_1", bytes.NewBufferString(patchBody))
	reqPatch.Header.Set("Content-Type", "application/json")
	wPatch := httptest.NewRecorder()
	r.ServeHTTP(wPatch, reqPatch)
	require.Equal(t, http.StatusOK, wPatch.Code)

	var patchResp struct {
		Scene api.SceneCard `json:"scene"`
	}
	require.NoError(t, json.Unmarshal(wPatch.Body.Bytes(), &patchResp))
	assert.Equal(t, "Updated moonlit forest", patchResp.Scene.Description)
	assert.Equal(t, 7.5, patchResp.Scene.DurationSec)
	assert.Equal(t, []string{"zoom_in", "leaves_fall"}, patchResp.Scene.Actions)

	// Reject invalid visual_type
	badPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/jobs/job-edit/storyboard/scenes/sc_1", bytes.NewBufferString(`{"visual_type":"invalid_type"}`))
	badPatch.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, badPatch)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)
}

func TestStoryboard_RegenerateAndStyle(t *testing.T) {
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	seedStoryboard(t, st, "job-regen")

	imgStub := &stubImageGen{}
	r := api.NewRouter(api.RouterDeps{
		Logger:           logger.New("error", "test"),
		Env:              "test",
		Storage:          st,
		ImageGen:         imgStub,
		MaxRegenAttempts: 1,
	})

	// List supported styles
	wStyles := httptest.NewRecorder()
	r.ServeHTTP(wStyles, httptest.NewRequest(http.MethodGet, "/api/v1/styles", nil))
	require.Equal(t, http.StatusOK, wStyles.Code)
	assert.Contains(t, wStyles.Body.String(), "anime")

	// Update job style
	styleReq := httptest.NewRequest(http.MethodPut, "/api/v1/jobs/job-regen/storyboard/style", bytes.NewBufferString(`{"style":"anime"}`))
	styleReq.Header.Set("Content-Type", "application/json")
	wStyle := httptest.NewRecorder()
	r.ServeHTTP(wStyle, styleReq)
	require.Equal(t, http.StatusOK, wStyle.Code)
	assert.Contains(t, wStyle.Body.String(), "anime")

	// Reject unsupported style
	badStyleReq := httptest.NewRequest(http.MethodPut, "/api/v1/jobs/job-regen/storyboard/style", bytes.NewBufferString(`{"style":"oil_painting"}`))
	badStyleReq.Header.Set("Content-Type", "application/json")
	wBadStyle := httptest.NewRecorder()
	r.ServeHTTP(wBadStyle, badStyleReq)
	assert.Equal(t, http.StatusBadRequest, wBadStyle.Code)

	// Regenerate scene 1st time (succeeds)
	regenReq := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/job-regen/storyboard/scenes/sc_1/regenerate", bytes.NewBufferString(`{"prompt":"custom anime forest"}`))
	regenReq.Header.Set("Content-Type", "application/json")
	wRegen := httptest.NewRecorder()
	r.ServeHTTP(wRegen, regenReq)
	require.Equal(t, http.StatusOK, wRegen.Code)
	assert.Equal(t, 1, imgStub.calls)

	// Regenerate scene 2nd time (exceeds MaxRegenAttempts=1 -> 429)
	regenReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/job-regen/storyboard/scenes/sc_1/regenerate", nil)
	wRegen2 := httptest.NewRecorder()
	r.ServeHTTP(wRegen2, regenReq2)
	assert.Equal(t, http.StatusTooManyRequests, wRegen2.Code)
}
