package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"reel/internal/config"
)

const openAIImagesEndpoint = "https://api.openai.com/v1/images/generations"

func init() {
	Register("openai", NewOpenAI)
}

type openAIClient struct {
	endpoint     string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewOpenAI creates an ImageGen Strategy backed by OpenAI DALL-E.
func NewOpenAI(cfg config.AIConfig) (Strategy, error) {
	if strings.TrimSpace(cfg.OpenAIAPIKey) == "" {
		return nil, errors.New("openai api key is required")
	}
	model := cfg.OpenAIImageGenModel
	if model == "" {
		model = cfg.ImageGenModel
	}
	if model == "" {
		model = "dall-e-3"
	}
	return &openAIClient{
		endpoint:     openAIImagesEndpoint,
		apiKey:       cfg.OpenAIAPIKey,
		defaultModel: model,
		httpClient:   sharedHTTPClient,
	}, nil
}

// NewOpenAIWithEndpoint allows overriding endpoint and httpClient in tests.
func NewOpenAIWithEndpoint(endpoint, apiKey, model string, hc *http.Client) Strategy {
	if hc == nil {
		hc = sharedHTTPClient
	}
	return &openAIClient{
		endpoint:     endpoint,
		apiKey:       apiKey,
		defaultModel: model,
		httpClient:   hc,
	}
}

func (o *openAIClient) Name() string { return "openai" }

func (o *openAIClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	model := req.Model
	if model == "" {
		model = o.defaultModel
	}

	size := "1024x1024"
	if strings.HasPrefix(model, "gpt-image") {
		if req.Width > req.Height {
			size = "1536x1024"
		} else if req.Height > req.Width {
			size = "1024x1536"
		}
	} else {
		if req.Width > req.Height {
			size = "1792x1024"
		} else if req.Height > req.Width {
			size = "1024x1792"
		}
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"model":  model,
		"prompt": req.Prompt,
		"n":      1,
		"size":   size,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai image request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageDownloadBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai image api error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode openai image response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, errors.New("openai returned empty image data")
	}

	var imgBytes []byte
	contentType := "image/png"
	if parsed.Data[0].B64JSON != "" {
		imgBytes, err = base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
		if err != nil {
			return nil, fmt.Errorf("decode b64_json: %w", err)
		}
	} else if parsed.Data[0].URL != "" {
		imgBytes, contentType, err = downloadImage(ctx, o.httpClient, parsed.Data[0].URL)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("openai returned neither b64_json nor url")
	}

	return &GenerateResponse{
		Provider:    "openai",
		Model:       model,
		ImageBytes:  imgBytes,
		ContentType: contentType,
	}, nil
}
