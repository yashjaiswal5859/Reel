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
	"time"
)

var sharedHTTPClient = &http.Client{
	Timeout: 2 * time.Minute,
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 24,
		IdleConnTimeout:     90 * time.Second,
	},
}

// openAICompatClient implements Strategy for OpenAI-compatible /chat/completions APIs
// (shared by Groq, OpenAI, and DeepSeek).
type openAICompatClient struct {
	providerName string
	endpoint     string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

func newOpenAICompatClient(providerName, endpoint, apiKey, defaultModel string, hc *http.Client) (*openAICompatClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("%s api key is required", providerName)
	}
	if hc == nil {
		hc = sharedHTTPClient
	}
	return &openAICompatClient{
		providerName: providerName,
		endpoint:     endpoint,
		apiKey:       apiKey,
		defaultModel: defaultModel,
		httpClient:   hc,
	}, nil
}

func (c *openAICompatClient) Name() string {
	return c.providerName
}

type openAIChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type openAIChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *openAICompatClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("messages cannot be empty")
	}
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}

	payload := openAIChatRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	if req.JSONMode {
		payload.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", c.providerName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", c.providerName, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", c.providerName, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", c.providerName, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s api error (%d): %s", c.providerName, resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", c.providerName, err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("%s returned empty choices", c.providerName)
	}

	usedModel := parsed.Model
	if usedModel == "" {
		usedModel = model
	}

	return &GenerateResponse{
		Provider: c.providerName,
		Model:    usedModel,
		Content:  parsed.Choices[0].Message.Content,
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		},
	}, nil
}
