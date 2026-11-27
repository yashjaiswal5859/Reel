package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"reel/internal/api"
	"reel/internal/logger"
	"reel/internal/media"
	"reel/internal/storage"
	"reel/internal/temporal/workflows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockJobService struct {
	mu   sync.RWMutex
	jobs map[string]*media.JobStatus
}

func newMockJobService() *mockJobService {
	return &mockJobService{jobs: make(map[string]*media.JobStatus)}
}

func (m *mockJobService) StartVideoJob(_ context.Context, input workflows.VideoJobInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[input.JobID] = &media.JobStatus{
		JobID:    input.JobID,
		Stage:    media.StagePreprocessing,
		Progress: 10,
		InputKey: input.InputKey,
	}
	return nil
}

func (m *mockJobService) GetJobStatus(_ context.Context, jobID string) (*media.JobStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.jobs[jobID]
	if !ok {
		return nil, assert.AnError
	}
	cp := *st
	return &cp, nil
}

func (m *mockJobService) CancelJob(_ context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.jobs[jobID]; ok {
		st.Stage = media.StageCancelled
	}
	return nil
}

func buildMultipartVideo(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("video", filename)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &body, w.FormDataContentType()
}

func TestJobUpload_Status_And_Cancel(t *testing.T) {
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	jobs := newMockJobService()

	r := api.NewRouter(api.RouterDeps{
		Logger:        logger.New("error", "test"),
		Env:           "test",
		Storage:       st,
		Jobs:          jobs,
		MaxFileSizeMB: 10,
	})

	// Valid MP4 header + dummy payload
	mp4Bytes := append([]byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, make([]byte, 64)...)
	body, ct := buildMultipartVideo(t, "story.mp4", mp4Bytes)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var created media.JobStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.NotEmpty(t, created.JobID)
	assert.Equal(t, media.StageUploaded, created.Stage)

	// Verify file exists in storage
	exists, err := st.Exists(context.Background(), created.InputKey)
	require.NoError(t, err)
	assert.True(t, exists)

	// Query status
	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+created.JobID, nil)
	statusW := httptest.NewRecorder()
	r.ServeHTTP(statusW, statusReq)
	require.Equal(t, http.StatusOK, statusW.Code)

	// Cancel job
	cancelReq := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+created.JobID, nil)
	cancelW := httptest.NewRecorder()
	r.ServeHTTP(cancelW, cancelReq)
	require.Equal(t, http.StatusOK, cancelW.Code)
}

func TestJobUpload_RejectsInvalidFile(t *testing.T) {
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	r := api.NewRouter(api.RouterDeps{
		Logger:        logger.New("error", "test"),
		Env:           "test",
		Storage:       st,
		Jobs:          newMockJobService(),
		MaxFileSizeMB: 10,
	})

	// Bad extension
	body, ct := buildMultipartVideo(t, "evil.exe", []byte("MZ-not-a-video"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/upload", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Valid extension but bad magic bytes
	body2, ct2 := buildMultipartVideo(t, "fake.mp4", []byte("plain text pretending to be mp4"))
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/upload", body2)
	req2.Header.Set("Content-Type", ct2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}
