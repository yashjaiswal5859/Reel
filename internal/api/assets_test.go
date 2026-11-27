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

func TestAssetStreaming_And_PathTraversalBlock(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, st.Put(ctx, "jobs/job-a/scenes/sc_1.png", bytes.NewReader([]byte("PNG-DATA")), 8, "image/png"))

	r := api.NewRouter(api.RouterDeps{
		Logger:  logger.New("error", "test"),
		Env:     "test",
		Storage: st,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-a/assets/scenes/sc_1.png", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.Equal(t, "PNG-DATA", w.Body.String())

	badReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-a/assets/../secret.txt", nil)
	badW := httptest.NewRecorder()
	r.ServeHTTP(badW, badReq)
	assert.Equal(t, http.StatusBadRequest, badW.Code)
}
