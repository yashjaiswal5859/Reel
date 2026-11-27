package composition_test

import (
	"testing"

	"reel/internal/composition"
	"reel/internal/media"

	"github.com/stretchr/testify/assert"
)

func TestLayoutStrategy_VerticalAndHorizontal(t *testing.T) {
	cfg := composition.DefaultExportConfig()
	strat := composition.NewLayoutStrategy(composition.LayoutVertical4060, cfg.AspectRatio)
	layout := strat.Compute(cfg)

	assert.Equal(t, 1080, layout.TotalWidth)
	assert.Equal(t, 1920, layout.TotalHeight)
	assert.False(t, layout.Horizontal)
	assert.Equal(t, 768, layout.Presenter.Height)
	assert.Equal(t, 1152, layout.Animation.Height)

	hCfg := composition.ExportConfig{
		AspectRatio: composition.Aspect16x9,
		Quality:     composition.Quality720p,
		Layout:      composition.LayoutHorizontal5050,
	}
	hLayout := composition.NewLayoutStrategy(hCfg.Layout, hCfg.AspectRatio).Compute(hCfg)
	assert.True(t, hLayout.Horizontal)
	assert.Equal(t, 1280, hLayout.TotalWidth)
	assert.Equal(t, 720, hLayout.TotalHeight)
	assert.Equal(t, 640, hLayout.Presenter.Width)

	fc := composition.BuildSplitFilter(layout, false, "subtitles.ass")
	assert.Contains(t, fc, "vstack=inputs=2")
	assert.Contains(t, fc, "[stacked]ass=subtitles.ass[vout]")
}

func TestValidateFinalVideo(t *testing.T) {
	cfg := composition.DefaultExportConfig()
	ok := composition.ValidateFinalVideo(&media.VideoMetadata{
		Width:       1080,
		Height:      1920,
		DurationSec: 30.2,
		HasAudio:    true,
	}, 30.0, cfg, 102400)
	assert.True(t, ok.Valid)

	bad := composition.ValidateFinalVideo(&media.VideoMetadata{
		Width:       720,
		Height:      1280,
		DurationSec: 15.0,
		HasAudio:    false,
	}, 30.0, cfg, 102400)
	assert.False(t, bad.Valid)
	assert.Len(t, bad.Errors, 3)
}
