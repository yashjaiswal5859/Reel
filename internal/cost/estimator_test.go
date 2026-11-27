package cost_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"reel/internal/cost"
)

func TestEstimate_DefaultsAndScaling(t *testing.T) {
	est := cost.Estimate(cost.EstimateRequest{
		DurationSec: 60,
	})

	assert.Equal(t, 60.0, est.Assumptions.DurationSec)
	assert.Equal(t, 8, est.Assumptions.SceneCount) // ceil(60 / 7.5)
	assert.Equal(t, 2, est.Assumptions.CharacterCount)
	assert.Equal(t, 10, est.Assumptions.TotalImages)
	assert.Equal(t, "1080p", est.Assumptions.Quality)
	assert.Greater(t, est.EstimatedUSD, 0.0)
	assert.Greater(t, est.EstimatedCredits, 0)

	est4K := cost.Estimate(cost.EstimateRequest{
		DurationSec:      60,
		SceneCount:       8,
		CharacterCount:   2,
		Quality:          "4k",
		ASRProvider:      "openai",
		LLMProvider:      "anthropic",
		ImageGenProvider: "openai",
	})
	assert.Greater(t, est4K.EstimatedUSD, est.EstimatedUSD)
	assert.Greater(t, est4K.Breakdown.ComputeUSD, est.Breakdown.ComputeUSD)
}

func TestEstimate_ZeroDuration(t *testing.T) {
	est := cost.Estimate(cost.EstimateRequest{DurationSec: 0})
	assert.Equal(t, 0.0, est.EstimatedUSD)
	assert.Equal(t, 0, est.EstimatedCredits)
	assert.Equal(t, 0, est.Assumptions.TotalImages)
}
