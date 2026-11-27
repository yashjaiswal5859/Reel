package asr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/ai/asr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroqWhisper_StreamingTranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.NoError(t, r.ParseMultipartForm(1024*1024))
		assert.Equal(t, "whisper-large-v3-turbo", r.FormValue("model"))
		assert.Equal(t, "verbose_json", r.FormValue("response_format"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"language": "en",
			"duration": 3.2,
			"text":     "Hello world. Next line!",
			"words": []map[string]any{
				{"word": "Hello", "start": 0.0, "end": 0.4},
				{"word": "world.", "start": 0.4, "end": 0.9},
				{"word": "Next", "start": 2.0, "end": 2.4},
				{"word": "line!", "start": 2.4, "end": 3.1},
			},
		})
	}))
	defer srv.Close()

	client, err := asr.NewGroqWhisperWithEndpoint(srv.URL, "test-key", "whisper-large-v3-turbo", srv.Client())
	require.NoError(t, err)
	assert.Equal(t, "groq", client.Name())

	resp, err := client.Transcribe(context.Background(), asr.TranscribeRequest{
		Audio:    bytes.NewReader([]byte("RIFF-fake-wav")),
		Filename: "audio.wav",
	})
	require.NoError(t, err)
	assert.Equal(t, "en", resp.Language)
	require.Len(t, resp.Segments, 2)
	assert.Equal(t, "Hello world.", resp.Segments[0].Text)
	assert.Equal(t, "Next line!", resp.Segments[1].Text)
}

func TestGroupWordsIntoSentences_PauseSplit(t *testing.T) {
	words := []asr.Word{
		{Text: "First", Start: 0.0, End: 0.3},
		{Text: "part", Start: 0.3, End: 0.6},
		// 1.2s pause -> triggers split even without punctuation
		{Text: "Second", Start: 1.8, End: 2.1},
		{Text: "part", Start: 2.1, End: 2.5},
	}

	segs := asr.GroupWordsIntoSentences(words)
	require.Len(t, segs, 2)
	assert.Equal(t, "First part", segs[0].Text)
	assert.Equal(t, "Second part", segs[1].Text)
}

func TestValidateTranscript(t *testing.T) {
	emptyReport := asr.ValidateTranscript(&asr.TranscribeResponse{})
	assert.False(t, emptyReport.Valid)

	validResp := &asr.TranscribeResponse{
		Segments: []asr.Segment{
			{
				Text:  "Hello world.",
				Start: 0.0,
				End:   1.0,
				Words: []asr.Word{
					{Text: "Hello", Start: 0.0, End: 0.4, Confidence: 0.95},
					{Text: "world.", Start: 0.4, End: 1.0, Confidence: 0.92},
				},
			},
		},
	}
	okReport := asr.ValidateTranscript(validResp)
	assert.True(t, okReport.Valid)
	assert.Empty(t, okReport.Issues)

	overlapResp := &asr.TranscribeResponse{
		Segments: []asr.Segment{
			{
				Text:  "Bad",
				Start: 0.0,
				End:   1.0,
				Words: []asr.Word{
					{Text: "A", Start: 0.0, End: 0.8},
					{Text: "B", Start: 0.5, End: 1.0}, // overlaps A
				},
			},
		},
	}
	badReport := asr.ValidateTranscript(overlapResp)
	assert.False(t, badReport.Valid)
}
