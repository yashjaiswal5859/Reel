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

const replicateBaseURL = "https://api.replicate.com/v1"

func init() {
	Register("replicate", NewReplicate)
}

type replicateClient struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewReplicate creates an ImageGen Strategy backed by Replicate (Flux / SDXL).
func NewReplicate(cfg config.AIConfig) (Strategy, error) {
	if strings.TrimSpace(cfg.ReplicateAPIKey) == "" {
		return nil, errors.New("replicate api key is required")
	}
	model := cfg.ReplicateImageGenModel
	if model == "" {
		model = cfg.ImageGenModel
	}
	if model == "" {
		model = "black-forest-labs/flux-1.1-pro"
	}
	return &replicateClient{
		baseURL:      replicateBaseURL,
		apiKey:       cfg.ReplicateAPIKey,
		defaultModel: model,
		httpClient:   sharedHTTPClient,
	}, nil
}

// NewReplicateWithBaseURL allows overriding baseURL and httpClient in tests.
func NewReplicateWithBaseURL(baseURL, apiKey, model string, hc *http.Client) Strategy {
	if hc == nil {
		hc = sharedHTTPClient
	}
	return &replicateClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   hc,
	}
}

func (r *replicateClient) Name() string { return "replicate" }

func (r *replicateClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	model := req.Model
	if model == "" {
		model = r.defaultModel
	}

	input := map[string]any{
		"prompt": req.Prompt,
	}
	if req.NegativePrompt != "" {
		input["negative_prompt"] = req.NegativePrompt
	}
	if req.Width > 0 && req.Height > 0 {
		input["width"] = req.Width
		input["height"] = req.Height
	}
	if req.Seed != nil {
		input["seed"] = *req.Seed
	}
	if len(req.ReferenceURLs) > 0 {
		input["image_prompt"] = req.ReferenceURLs[0]
	}

	payloadBytes, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/models/%s/predictions", r.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Prefer", "wait")

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("replicate request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("replicate api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Output any    `json:"output"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode replicate response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("replicate prediction error: %s", parsed.Error)
	}

	imgURL := extractFirstURL(parsed.Output)
	if imgURL == "" {
		return nil, errors.New("replicate returned no output image url")
	}

	imgBytes, ct, err := downloadImage(ctx, r.httpClient, imgURL)
	if err != nil {
		return nil, err
	}

	return &GenerateResponse{
		Provider:    "replicate",
		Model:       model,
		ImageBytes:  imgBytes,
		ImageURL:    imgURL,
		ContentType: ct,
	}, nil
}

func extractFirstURL(out any) string {
	switch v := out.(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}
