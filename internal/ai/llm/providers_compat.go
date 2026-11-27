package llm

import (
	"net/http"

	"reel/internal/config"
)

const (
	groqEndpoint     = "https://api.groq.com/openai/v1/chat/completions"
	openAIEndpoint   = "https://api.openai.com/v1/chat/completions"
	deepSeekEndpoint = "https://api.deepseek.com/chat/completions"
)

func init() {
	Register("groq", NewGroq)
	Register("openai", NewOpenAI)
	Register("deepseek", NewDeepSeek)
}

// NewGroq creates an LLM Strategy backed by Groq.
func NewGroq(cfg config.AIConfig) (Strategy, error) {
	model := firstNonEmpty(cfg.GroqLLMModel, cfg.LLMModel, "llama-3.3-70b-versatile")
	return newOpenAICompatClient("groq", groqEndpoint, cfg.GroqAPIKey, model, nil)
}

// NewOpenAI creates an LLM Strategy backed by OpenAI.
func NewOpenAI(cfg config.AIConfig) (Strategy, error) {
	model := firstNonEmpty(cfg.OpenAILLMModel, cfg.LLMModel, "gpt-4o")
	return newOpenAICompatClient("openai", openAIEndpoint, cfg.OpenAIAPIKey, model, nil)
}

// NewDeepSeek creates an LLM Strategy backed by DeepSeek.
func NewDeepSeek(cfg config.AIConfig) (Strategy, error) {
	model := firstNonEmpty(cfg.DeepSeekLLMModel, cfg.LLMModel, "deepseek-chat")
	return newOpenAICompatClient("deepseek", deepSeekEndpoint, cfg.DeepSeekAPIKey, model, nil)
}

// NewOpenAICompatWithEndpoint creates a testable OpenAI-compatible LLM client.
func NewOpenAICompatWithEndpoint(provider, endpoint, apiKey, model string, hc *http.Client) (Strategy, error) {
	return newOpenAICompatClient(provider, endpoint, apiKey, model, hc)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
