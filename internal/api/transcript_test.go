package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/ai/asr"
	"reel/internal/api"
	"reel/internal/logger"
	"reel/internal/storage"
	"reel/internal/temporal/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranscript_GetAndUpdate(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	doc := activities.StoredTranscript{
		Transcript: &asr.TranscribeResponse{
			Provider: "groq",
			Language: "en",
			Text:     "Helo world.",
			Segments: []asr.Segment{
				{
					Text:  "Helo world.",
					Start: 0.0,
					End:   1.0,
					Words: []asr.Word{
						{Text: "Helo", Start: 0.0, End: 0.4},
						{Text: "world.", Start: 0.4, End: 1.0},
					},
				},
			},
		},
		Quality: asr.QualityReport{Valid: true},
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, st.Put(ctx, "jobs/job-42/transcript.json", bytes.NewReader(raw), int64(len(raw)), "application/json"))

	r := api.NewRouter(api.RouterDeps{
		Logger:  logger.New("error", "test"),
		Env:     "test",
		Storage: st,
	})

	// GET existing transcript
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-42/transcript", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	// PUT edited transcript (fix typo "Helo" -> "Hello")
	doc.Transcript.Segments[0].Words[0].Text = "Hello"
	editBody, err := json.Marshal(doc.Transcript)
	require.NoError(t, err)

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/jobs/job-42/transcript", bytes.NewReader(editBody))
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	require.Equal(t, http.StatusOK, putW.Code)

	var updated activities.StoredTranscript
	require.NoError(t, json.Unmarshal(putW.Body.Bytes(), &updated))
	assert.Equal(t, "Hello world.", updated.Transcript.Text)
	assert.Equal(t, "Hello world.", updated.Transcript.Segments[0].Text)
	assert.True(t, updated.Quality.Valid)

	// GET missing transcript
	missReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-missing/transcript", nil)
	missW := httptest.NewRecorder()
	r.ServeHTTP(missW, missReq)
	assert.Equal(t, http.StatusNotFound, missW.Code)
}
