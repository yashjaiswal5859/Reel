package media_test

import (
	"testing"

	"reel/internal/media"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateExtension(t *testing.T) {
	require.NoError(t, media.ValidateExtension("clip.mp4"))
	require.NoError(t, media.ValidateExtension("CLIP.MOV"))
	require.NoError(t, media.ValidateExtension("story.webm"))
	require.Error(t, media.ValidateExtension("script.exe"))
	require.Error(t, media.ValidateExtension("noext"))
}

func TestValidateVideoHeader(t *testing.T) {
	mp4 := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	require.NoError(t, media.ValidateVideoHeader(mp4))

	webm := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0xF7, 0x81}
	require.NoError(t, media.ValidateVideoHeader(webm))

	require.Error(t, media.ValidateVideoHeader([]byte("not a video file at all")))
	require.Error(t, media.ValidateVideoHeader([]byte("short")))
}

func TestDetectOrientation(t *testing.T) {
	assert.Equal(t, "landscape", media.DetectOrientation(1920, 1080))
	assert.Equal(t, "portrait", media.DetectOrientation(1080, 1920))
	assert.Equal(t, "square", media.DetectOrientation(1080, 1080))
}
