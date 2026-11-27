package videogen

import "context"

// GenerateRequest describes an image-to-video generation request (P3).
type GenerateRequest struct {
	Prompt         string  `json:"prompt"`
	SourceImageURL string  `json:"source_image_url"`
	DurationSec    float64 `json:"duration_sec"`
	Model          string  `json:"model,omitempty"`
}

// GenerateResponse contains the generated video clip URL or bytes.
type GenerateResponse struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	VideoURL   string `json:"video_url,omitempty"`
	VideoBytes []byte `json:"-"`
}

// Strategy defines the AI Video Generation provider contract.
type Strategy interface {
	Name() string
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}
