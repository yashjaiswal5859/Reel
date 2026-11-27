package imagegen

import "context"

// GenerateRequest describes an image generation prompt and parameters.
type GenerateRequest struct {
	Prompt         string   `json:"prompt"`
	NegativePrompt string   `json:"negative_prompt,omitempty"`
	Model          string   `json:"model,omitempty"`
	Width          int      `json:"width,omitempty"`
	Height         int      `json:"height,omitempty"`
	ReferenceURLs  []string `json:"reference_urls,omitempty"`
	Seed           *int64   `json:"seed,omitempty"`
}

// GenerateResponse contains the generated image bytes or URL.
type GenerateResponse struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	ImageBytes  []byte `json:"-"`
	ImageURL    string `json:"image_url,omitempty"`
	ContentType string `json:"content_type"`
}

// Strategy defines the Image Generation provider contract.
type Strategy interface {
	Name() string
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}
