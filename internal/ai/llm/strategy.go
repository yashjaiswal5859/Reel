package llm

import "context"

// Message represents a chat message in an LLM conversation.
type Message struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// GenerateRequest contains prompt messages and generation parameters.
type GenerateRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	JSONMode    bool      `json:"json_mode,omitempty"`
}

// Usage tracks token consumption for cost estimation and limits.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// GenerateResponse contains the model's output text and usage stats.
type GenerateResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Content  string `json:"content"`
	Usage    Usage  `json:"usage"`
}

// Strategy defines the LLM provider contract.
type Strategy interface {
	Name() string
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}
