package cost

import (
	"math"
	"strings"
)

const (
	// CreditValueUSD defines how much one platform credit represents in USD.
	CreditValueUSD = 0.01
	// DefaultSecondsPerScene is used to project scene count when not explicitly given.
	DefaultSecondsPerScene = 7.5
	// DefaultCharacterCount is used to project character reference sheets when unknown.
	DefaultCharacterCount = 2
)

// EstimateRequest specifies parameters for pre-processing or job cost estimation.
type EstimateRequest struct {
	DurationSec      float64 `json:"duration_sec"`
	SceneCount       int     `json:"scene_count,omitempty"`
	CharacterCount   int     `json:"character_count,omitempty"`
	Quality          string  `json:"quality,omitempty"`           // "720p" | "1080p" | "4k"
	ASRProvider      string  `json:"asr_provider,omitempty"`      // "groq" | "openai" | "deepgram"
	LLMProvider      string  `json:"llm_provider,omitempty"`      // "groq" | "openai" | "gemini" | "anthropic" | "deepseek"
	ImageGenProvider string  `json:"imagegen_provider,omitempty"` // "replicate" | "fal" | "openai"
	LLMTokensUsed    int     `json:"llm_tokens_used,omitempty"`   // Optional actual token usage
}

// CostBreakdown itemizes estimated USD costs per pipeline component.
type CostBreakdown struct {
	ASRUSD      float64 `json:"asr_usd"`
	LLMUSD      float64 `json:"llm_usd"`
	ImageGenUSD float64 `json:"imagegen_usd"`
	ComputeUSD  float64 `json:"compute_usd"`
}

// CostAssumptions documents the inputs and defaults used for the estimate.
type CostAssumptions struct {
	DurationSec      float64 `json:"duration_sec"`
	SceneCount       int     `json:"scene_count"`
	CharacterCount   int     `json:"character_count"`
	Quality          string  `json:"quality"`
	ASRProvider      string  `json:"asr_provider"`
	LLMProvider      string  `json:"llm_provider"`
	ImageGenProvider string  `json:"imagegen_provider"`
	TotalImages      int     `json:"total_images"`
}

// CostEstimate is the complete cost projection in USD and platform credits.
type CostEstimate struct {
	EstimatedUSD     float64         `json:"estimated_usd"`
	EstimatedCredits int             `json:"estimated_credits"`
	Breakdown        CostBreakdown   `json:"breakdown"`
	Assumptions      CostAssumptions `json:"assumptions"`
}

// asrRatePerMin returns approximate USD per audio minute.
var asrRatePerMin = map[string]float64{
	"groq":     0.0007, // Whisper large-v3-turbo on Groq
	"openai":   0.0060, // OpenAI Whisper-1
	"deepgram": 0.0043, // Deepgram Nova-2
}

// llmRatePer1KTokens returns blended USD per 1K tokens.
var llmRatePer1KTokens = map[string]float64{
	"groq":      0.0007,
	"openai":    0.0050,
	"gemini":    0.0004,
	"anthropic": 0.0060,
	"deepseek":  0.0005,
}

// imageRatePerImage returns approximate USD per generated image.
var imageRatePerImage = map[string]float64{
	"replicate": 0.040, // Flux 1.1 Pro
	"fal":       0.035, // fal.ai Flux
	"openai":    0.040, // DALL-E 3 standard
}

// qualityMultiplier scales FFmpeg compute cost by export resolution.
var qualityMultiplier = map[string]float64{
	"720p":  1.0,
	"1080p": 1.5,
	"4k":    3.0,
}

// Estimate computes projected processing cost from duration, scene/character counts, and providers.
func Estimate(req EstimateRequest) CostEstimate {
	dur := math.Max(0, req.DurationSec)
	minutes := dur / 60.0

	scenes := req.SceneCount
	if scenes <= 0 && dur > 0 {
		scenes = int(math.Ceil(dur / DefaultSecondsPerScene))
		if scenes < 1 {
			scenes = 1
		}
	}

	chars := req.CharacterCount
	if chars < 0 {
		chars = 0
	} else if chars == 0 && dur > 0 {
		chars = DefaultCharacterCount
	}

	asrProv := normalizeProvider(req.ASRProvider, "groq")
	llmProv := normalizeProvider(req.LLMProvider, "groq")
	imgProv := normalizeProvider(req.ImageGenProvider, "replicate")
	quality := strings.ToLower(strings.TrimSpace(req.Quality))
	if _, ok := qualityMultiplier[quality]; !ok {
		quality = "1080p"
	}

	asrRate := lookupOrDefault(asrRatePerMin, asrProv, 0.0060)
	llmRate := lookupOrDefault(llmRatePer1KTokens, llmProv, 0.0020)
	imgRate := lookupOrDefault(imageRatePerImage, imgProv, 0.040)
	qMult := qualityMultiplier[quality]

	asrUSD := minutes * asrRate

	// Estimate ~2500 tokens per minute of narration for story analysis + storyboard when actuals aren't available.
	tokens := float64(req.LLMTokensUsed)
	if tokens <= 0 && dur > 0 {
		tokens = math.Max(1500, minutes*2500)
	}
	llmUSD := (tokens / 1000.0) * llmRate

	totalImages := scenes + chars
	imgUSD := float64(totalImages) * imgRate

	// Base FFmpeg rendering/compositing compute ~$0.01/min * quality multiplier.
	computeUSD := minutes * 0.01 * qMult

	total := roundUSD(asrUSD + llmUSD + imgUSD + computeUSD)
	credits := 0
	if total > 0 {
		credits = int(math.Ceil(total / CreditValueUSD))
		if credits < 1 {
			credits = 1
		}
	}

	return CostEstimate{
		EstimatedUSD:     total,
		EstimatedCredits: credits,
		Breakdown: CostBreakdown{
			ASRUSD:      roundUSD(asrUSD),
			LLMUSD:      roundUSD(llmUSD),
			ImageGenUSD: roundUSD(imgUSD),
			ComputeUSD:  roundUSD(computeUSD),
		},
		Assumptions: CostAssumptions{
			DurationSec:      dur,
			SceneCount:       scenes,
			CharacterCount:   chars,
			Quality:          quality,
			ASRProvider:      asrProv,
			LLMProvider:      llmProv,
			ImageGenProvider: imgProv,
			TotalImages:      totalImages,
		},
	}
}

func normalizeProvider(val, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(val))
	if s == "" {
		return fallback
	}
	return s
}

func lookupOrDefault(m map[string]float64, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func roundUSD(v float64) float64 {
	return math.Round(v*10000) / 10000
}
