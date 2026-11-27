package asr

import (
	"net/http"

	"reel/internal/config"
)

const openAITranscribeEndpoint = "https://api.openai.com/v1/audio/transcriptions"

func init() {
	Register("openai", NewOpenAIWhisper)
}

// NewOpenAIWhisper creates an ASR Strategy backed by OpenAI's Whisper API.
func NewOpenAIWhisper(cfg config.AIConfig) (Strategy, error) {
	model := cfg.OpenAIASRModel
	if model == "" {
		model = cfg.ASRModel
	}
	if model == "" {
		model = "whisper-1"
	}
	return newOpenAICompatClient("openai", openAITranscribeEndpoint, cfg.OpenAIAPIKey, model, nil)
}

// NewOpenAIWhisperWithEndpoint allows overriding the endpoint and HTTP client in tests.
func NewOpenAIWhisperWithEndpoint(endpoint, apiKey, model string, httpClient *http.Client) (Strategy, error) {
	return newOpenAICompatClient("openai", endpoint, apiKey, model, httpClient)
}
