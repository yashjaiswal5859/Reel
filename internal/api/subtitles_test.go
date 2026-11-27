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
	"reel/internal/subtitles"
	"reel/internal/temporal/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubtitles_GetAndUpdateStyle(t *testing.T) {
	ctx := context.Background()
	st, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	doc := activities.StoredTranscript{
		Transcript: &asr.TranscribeResponse{
			Segments: []asr.Segment{
				{
					Text:  "One two three four.",
					Start: 0.0,
					End:   2.0,
					Words: []asr.Word{
						{Text: "One", Start: 0.0, End: 0.5},
						{Text: "two", Start: 0.5, End: 1.0},
						{Text: "three", Start: 1.0, End: 1.5},
						{Text: "four.", Start: 1.5, End: 2.0},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, st.Put(ctx, "jobs/job-sub/transcript.json", bytes.NewReader(raw), int64(len(raw)), "application/json"))

	r := api.NewRouter(api.RouterDeps{
		Logger:  logger.New("error", "test"),
		Env:     "test",
		Storage: st,
	})

	// Update style -> generates subtitles.json and subtitles.ass
	style := subtitles.DefaultStyle()
	style.Template = "bounce"
	style.MaxWordsPerLine = 2
	styleBody, err := json.Marshal(style)
	require.NoError(t, err)

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/jobs/job-sub/subtitles/style", bytes.NewReader(styleBody))
	putReq.Header.Set("Content-Type", "application/json")
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	require.Equal(t, http.StatusOK, putW.Code)

	var track subtitles.Track
	require.NoError(t, json.Unmarshal(putW.Body.Bytes(), &track))
	assert.Equal(t, "bounce", track.Style.Template)
	assert.Len(t, track.Cues, 2)

	// GET subtitles
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/job-sub/subtitles", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	// Verify .ass file was also generated
	hasASS, err := st.Exists(ctx, "jobs/job-sub/subtitles.ass")
	require.NoError(t, err)
	assert.True(t, hasASS)
}
