package media_test

import (
	"testing"

	"reel/internal/media"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFFprobeJSON(t *testing.T) {
	sample := []byte(`{
		"format": {"duration": "42.5"},
		"streams": [
			{"codec_type": "video", "codec_name": "h264", "width": 1080, "height": 1920, "r_frame_rate": "30000/1001"},
			{"codec_type": "audio", "codec_name": "aac"}
		]
	}`)

	meta, err := media.ParseFFprobeJSON(sample)
	require.NoError(t, err)
	assert.InDelta(t, 42.5, meta.DurationSec, 0.01)
	assert.Equal(t, 1080, meta.Width)
	assert.Equal(t, 1920, meta.Height)
	assert.Equal(t, "portrait", meta.Orientation)
	assert.Equal(t, "h264", meta.Codec)
	assert.InDelta(t, 29.97, meta.FPS, 0.01)
	assert.True(t, meta.HasAudio)
}

func TestParseFFprobeJSON_NoVideoStream(t *testing.T) {
	sample := []byte(`{"format": {"duration": "10.0"}, "streams": [{"codec_type": "audio"}]}`)
	_, err := media.ParseFFprobeJSON(sample)
	require.Error(t, err)
}
