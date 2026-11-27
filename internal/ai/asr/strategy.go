package asr

import (
	"context"
	"io"
)

// Word represents a single transcribed word with start/end timestamps in seconds.
type Word struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence,omitempty"`
}

// Segment represents a grouped sentence/phrase with its words.
type Segment struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Words []Word  `json:"words"`
}

// TranscribeRequest contains input audio and options for ASR.
type TranscribeRequest struct {
	Audio    io.Reader
	Filename string
	Language string // optional ISO-639-1 hint; empty for auto-detect
	Model    string // optional override
}

// TranscribeResponse contains the normalized transcript and word timestamps.
type TranscribeResponse struct {
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Language string    `json:"language"`
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Duration float64   `json:"duration"`
}

// Strategy defines the Speech-to-Text provider contract.
type Strategy interface {
	Name() string
	Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error)
}
