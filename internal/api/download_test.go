package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/api"
	"reel/internal/logger"
	"reel/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadFinalVideo(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, st.Put(ctx, "jobs/job-done/final.mp4", bytes.NewReader([]byte("FINAL-MP4")), 9, "video/mp4"))

	r := api.NewRouter(api.RouterDeps{
		Logger:  logger.New("error", "test"),
		Env:     "test",
		Storage: st,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-done/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "reel-job-done.mp4")
	assert.Equal(t, "FINAL-MP4", w.Body.String())

	missReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-pending/download", nil)
	missW := httptest.NewRecorder()
	r.ServeHTTP(missW, missReq)
	assert.Equal(t, http.StatusNotFound, missW.Code)
}
