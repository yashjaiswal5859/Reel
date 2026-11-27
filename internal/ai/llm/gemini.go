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

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

func init() {
	Register("gemini", NewGemini)
}

type geminiClient struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewGemini creates an LLM Strategy backed by Google Gemini.
func NewGemini(cfg config.AIConfig) (Strategy, error) {
	if strings.TrimSpace(cfg.GeminiAPIKey) == "" {
		return nil, errors.New("gemini api key is required")
	}
	model := firstNonEmpty(cfg.GeminiLLMModel, cfg.LLMModel, "gemini-2.0-flash")
	return &geminiClient{
		baseURL:      geminiBaseURL,
		apiKey:       cfg.GeminiAPIKey,
		defaultModel: model,
		httpClient:   sharedHTTPClient,
	}, nil
}

// NewGeminiWithBaseURL allows overriding baseURL and httpClient in tests.
func NewGeminiWithBaseURL(baseURL, apiKey, model string, hc *http.Client) Strategy {
	if hc == nil {
		hc = sharedHTTPClient
	}
	return &geminiClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   hc,
	}
}

func (g *geminiClient) Name() string { return "gemini" }

type geminiRequest struct {
	SystemInstruction *geminiContent    `json:"systemInstruction,omitempty"`
	Contents          []geminiContent   `json:"contents"`
	GenerationConfig  *geminiGenConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (g *geminiClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("messages cannot be empty")
	}
	model := firstNonEmpty(req.Model, g.defaultModel)

	var sysInstr *geminiContent
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			sysInstr = &geminiContent{Parts: []geminiPart{{Text: m.Content}}}
			continue
		}
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	genCfg := &geminiGenConfig{
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxTokens,
	}
	if req.JSONMode {
		genCfg.ResponseMimeType = "application/json"
	}

	bodyBytes, err := json.Marshal(geminiRequest{
		SystemInstruction: sysInstr,
		Contents:          contents,
		GenerationConfig:  genCfg,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s", g.baseURL, model, g.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("gemini returned empty candidates")
	}

	return &GenerateResponse{
		Provider: "gemini",
		Model:    model,
		Content:  parsed.Candidates[0].Content.Parts[0].Text,
		Usage: Usage{
			PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
			CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      parsed.UsageMetadata.TotalTokenCount,
		},
	}, nil
}
