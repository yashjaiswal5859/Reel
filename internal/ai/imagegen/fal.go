package imagegen

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

const falBaseURL = "https://fal.run"

func init() {
	Register("fal", NewFal)
}

type falClient struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewFal creates an ImageGen Strategy backed by fal.ai.
func NewFal(cfg config.AIConfig) (Strategy, error) {
	if strings.TrimSpace(cfg.FalAPIKey) == "" {
		return nil, errors.New("fal api key is required")
	}
	model := cfg.FalImageGenModel
	if model == "" {
		model = cfg.ImageGenModel
	}
	if model == "" {
		model = "fal-ai/flux-pro/v1.1"
	}
	return &falClient{
		baseURL:      falBaseURL,
		apiKey:       cfg.FalAPIKey,
		defaultModel: model,
		httpClient:   sharedHTTPClient,
	}, nil
}

// NewFalWithBaseURL allows overriding baseURL and httpClient in tests.
func NewFalWithBaseURL(baseURL, apiKey, model string, hc *http.Client) Strategy {
	if hc == nil {
		hc = sharedHTTPClient
	}
	return &falClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   hc,
	}
}

func (f *falClient) Name() string { return "fal" }

func (f *falClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	model := req.Model
	if model == "" {
		model = f.defaultModel
	}

	bodyMap := map[string]any{
		"prompt": req.Prompt,
	}
	if req.NegativePrompt != "" {
		bodyMap["negative_prompt"] = req.NegativePrompt
	}
	if req.Width > 0 && req.Height > 0 {
		bodyMap["image_size"] = map[string]int{"width": req.Width, "height": req.Height}
	}
	if req.Seed != nil {
		bodyMap["seed"] = *req.Seed
	}

	payloadBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s", f.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Key "+f.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fal request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fal api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Images []struct {
			URL         string `json:"url"`
			ContentType string `json:"content_type"`
		} `json:"images"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode fal response: %w", err)
	}
	if len(parsed.Images) == 0 || parsed.Images[0].URL == "" {
		return nil, errors.New("fal returned no images")
	}

	imgBytes, ct, err := downloadImage(ctx, f.httpClient, parsed.Images[0].URL)
	if err != nil {
		return nil, err
	}

	return &GenerateResponse{
		Provider:    "fal",
		Model:       model,
		ImageBytes:  imgBytes,
		ImageURL:    parsed.Images[0].URL,
		ContentType: ct,
	}, nil
}
