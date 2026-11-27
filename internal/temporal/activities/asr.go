package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"go.temporal.io/sdk/temporal"
	"reel/internal/ai/asr"
	"reel/internal/storage"
)

// ASRActivities implements Temporal activities for speech-to-text transcription.
type ASRActivities struct {
	Storage storage.Strategy
	ASR     asr.Strategy
}

// TranscribeInput identifies the extracted audio to transcribe.
type TranscribeInput struct {
	JobID    string `json:"job_id"`
	AudioKey string `json:"audio_key"`
	Language string `json:"language,omitempty"`
}

// TranscribeOutput holds the stored transcript key and quality summary.
type TranscribeOutput struct {
	TranscriptKey string            `json:"transcript_key"`
	Language      string            `json:"language"`
	SegmentCount  int               `json:"segment_count"`
	Quality       asr.QualityReport `json:"quality"`
}

// StoredTranscript is the JSON artifact saved to temporary storage.
type StoredTranscript struct {
	Transcript *asr.TranscribeResponse `json:"transcript"`
	Quality    asr.QualityReport       `json:"quality"`
}

// TranscribeAudio streams audio from storage to the configured ASR provider and saves transcript.json.
func (a *ASRActivities) TranscribeAudio(ctx context.Context, input TranscribeInput) (*TranscribeOutput, error) {
	if a.ASR == nil {
		return nil, temporal.NewNonRetryableApplicationError("asr provider not configured", "ConfigError", nil)
	}

	rc, err := a.Storage.Get(ctx, input.AudioKey)
	if err != nil {
		return nil, fmt.Errorf("get audio %s: %w", input.AudioKey, err)
	}
	defer rc.Close()

	resp, err := a.ASR.Transcribe(ctx, asr.TranscribeRequest{
		Audio:    rc,
		Filename: "audio.wav",
		Language: input.Language,
	})
	if err != nil {
		return nil, fmt.Errorf("asr transcribe: %w", err)
	}

	quality := asr.ValidateTranscript(resp)
	doc := StoredTranscript{
		Transcript: resp,
		Quality:    quality,
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal transcript: %w", err)
	}

	transcriptKey := fmt.Sprintf("jobs/%s/transcript.json", input.JobID)
	if err := a.Storage.Put(ctx, transcriptKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		return nil, fmt.Errorf("store transcript: %w", err)
	}

	return &TranscribeOutput{
		TranscriptKey: transcriptKey,
		Language:      resp.Language,
		SegmentCount:  len(resp.Segments),
		Quality:       quality,
	}, nil
}
