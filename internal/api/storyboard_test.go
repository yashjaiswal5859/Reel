package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/api"
	"reel/internal/logger"
	"reel/internal/storage"
	"reel/internal/story"
	"reel/internal/temporal/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoryboard_Get(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	doc := activities.StoredStoryboard{
		Provider: "groq",
		Model:    "llama-3.3-70b-versatile",
		Quality:  story.QualityReport{Valid: true},
		Storyboard: &story.Storyboard{
			Style: story.Style2DCartoon,
			Scenes: []story.Scene{
				{SceneID: "sc_1", Index: 1, StartSec: 0, EndSec: 5, Description: "Forest intro"},
			},
		},
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, st.Put(ctx, "jobs/job-sb/storyboard.json", bytes.NewReader(raw), int64(len(raw)), "application/json"))

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
