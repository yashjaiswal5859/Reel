package vision

import "context"

// AnalyzeRequest contains one or more images and an evaluation/analysis prompt.
type AnalyzeRequest struct {
	Prompt     string   `json:"prompt"`
	ImageURLs  []string `json:"image_urls,omitempty"`
	ImageBytes [][]byte `json:"-"`
	Model      string   `json:"model,omitempty"`
	JSONMode   bool     `json:"json_mode,omitempty"`
}

// AnalyzeResponse contains the vision model's analysis.
type AnalyzeResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Content  string `json:"content"`
}

// Strategy defines the Vision/Multimodal analysis provider contract.
type Strategy interface {
	Name() string
	Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error)
}
