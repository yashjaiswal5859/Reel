package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"reel/internal/config"
)

const anthropicEndpoint = "https://api.anthropic.com/v1/messages"

func init() {
	Register("anthropic", NewAnthropic)
}

type anthropicClient struct {
	endpoint     string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewAnthropic creates an LLM Strategy backed by Anthropic Claude.
func NewAnthropic(cfg config.AIConfig) (Strategy, error) {
	if strings.TrimSpace(cfg.AnthropicAPIKey) == "" {
		return nil, errors.New("anthropic api key is required")
	}
	model := firstNonEmpty(cfg.AnthropicLLMModel, cfg.LLMModel, "claude-sonnet-4-20250514")
	return &anthropicClient{
		endpoint:     anthropicEndpoint,
		apiKey:       cfg.AnthropicAPIKey,
		defaultModel: model,
		httpClient:   sharedHTTPClient,
	}, nil
}

// NewAnthropicWithEndpoint allows overriding endpoint and httpClient in tests.
func NewAnthropicWithEndpoint(endpoint, apiKey, model string, hc *http.Client) Strategy {
	if hc == nil {
		hc = sharedHTTPClient
	}
	return &anthropicClient{
		endpoint:     endpoint,
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   hc,
	}
}

func (a *anthropicClient) Name() string { return "anthropic" }

type anthropicRequest struct {
	Model       string    `json:"model"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (a *anthropicClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("messages cannot be empty")
	}
	model := firstNonEmpty(req.Model, a.defaultModel)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	var sysPrompt string
	msgs := make([]Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			sysPrompt = m.Content
			continue
		}
		msgs = append(msgs, m)
	}

	bodyBytes, err := json.Marshal(anthropicRequest{
		Model:       model,
		System:      sysPrompt,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	if len(parsed.Content) == 0 {
		return nil, errors.New("anthropic returned empty content")
	}

	return &GenerateResponse{
		Provider: "anthropic",
		Model:    firstNonEmpty(parsed.Model, model),
		Content:  parsed.Content[0].Text,
		Usage: Usage{
			PromptTokens:     parsed.Usage.InputTokens,
			CompletionTokens: parsed.Usage.OutputTokens,
			TotalTokens:      parsed.Usage.InputTokens + parsed.Usage.OutputTokens,
		},
	}, nil
}
