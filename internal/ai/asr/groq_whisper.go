package asr

import (
	"net/http"

	"reel/internal/config"
)

const groqTranscribeEndpoint = "https://api.groq.com/openai/v1/audio/transcriptions"

func init() {
	Register("groq", NewGroqWhisper)
}

// NewGroqWhisper creates an ASR Strategy backed by Groq's Whisper API.
func NewGroqWhisper(cfg config.AIConfig) (Strategy, error) {
	model := cfg.GroqASRModel
	if model == "" {
		model = cfg.ASRModel
	}
	if model == "" {
		model = "whisper-large-v3-turbo"
	}
	return newOpenAICompatClient("groq", groqTranscribeEndpoint, cfg.GroqAPIKey, model, nil)
}

// NewGroqWhisperWithEndpoint allows overriding the endpoint and HTTP client in tests.
func NewGroqWhisperWithEndpoint(endpoint, apiKey, model string, httpClient *http.Client) (Strategy, error) {
	return newOpenAICompatClient("groq", endpoint, apiKey, model, httpClient)
}
