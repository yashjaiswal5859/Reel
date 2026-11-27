package subtitles_test

import (
	"bytes"
	"testing"

	"reel/internal/ai/asr"
	"reel/internal/media"
	"reel/internal/subtitles"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTrack_ChunkingAndKaraoke(t *testing.T) {
	transcript := &asr.TranscribeResponse{
		Segments: []asr.Segment{
			{
				Text:  "One two three four five six.",
				Start: 0.0,
				End:   3.0,
				Words: []asr.Word{
					{Text: "One", Start: 0.0, End: 0.4},
					{Text: "two", Start: 0.4, End: 0.8},
					{Text: "three", Start: 0.8, End: 1.2},
					{Text: "four", Start: 1.2, End: 1.6},
					{Text: "five", Start: 1.6, End: 2.2},
					{Text: "six.", Start: 2.2, End: 3.0},
				},
			},
		},
	}

	style := subtitles.DefaultStyle()
	style.MaxWordsPerLine = 3

	track, err := subtitles.GenerateTrack(transcript, style)
	require.NoError(t, err)
	require.Len(t, track.Cues, 2)
	assert.Equal(t, "One two three", track.Cues[0].Text)
	assert.Equal(t, "four five six.", track.Cues[1].Text)
	assert.Equal(t, 40, track.Cues[0].Words[0].DurationCS)
	assert.Equal(t, 80, track.Cues[1].Words[2].DurationCS)
}

func TestComputePlacement_Orientations(t *testing.T) {
	style := subtitles.DefaultStyle()

	portrait := subtitles.ComputePlacement(&media.VideoMetadata{Width: 1080, Height: 1920, Orientation: "portrait"}, style)
	assert.Equal(t, 2, portrait.ASSAlign)
	assert.Equal(t, 56, portrait.FontSizePx)

	landscape := subtitles.ComputePlacement(&media.VideoMetadata{Width: 1920, Height: 1080, Orientation: "landscape"}, style)
	assert.Equal(t, 1920, landscape.PlayResX)
	assert.Greater(t, landscape.FontSizePx, 56)
}

func TestRenderASS_AllTemplates(t *testing.T) {
	transcript := &asr.TranscribeResponse{
		Segments: []asr.Segment{
			{
				Text:  "Hello world!",
				Start: 0.0,
				End:   1.2,
				Words: []asr.Word{
					{Text: "Hello", Start: 0.0, End: 0.5},
					{Text: "world!", Start: 0.5, End: 1.2},
				},
			},
		},
	}

	meta := &media.VideoMetadata{Width: 1080, Height: 1920, Orientation: "portrait"}

	for _, tmpl := range subtitles.SupportedTemplates() {
		style := subtitles.DefaultStyle()
		style.Template = tmpl

		track, err := subtitles.GenerateTrack(transcript, style)
		require.NoError(t, err)

		var buf bytes.Buffer
		require.NoError(t, subtitles.RenderASS(&buf, track, meta))
		out := buf.String()
		assert.Contains(t, out, "[Script Info]")
		assert.Contains(t, out, "[V4+ Styles]")
		assert.Contains(t, out, "Dialogue: 0,0:00:00.00,0:00:01.20,Default")
		assert.Contains(t, out, "Hello")
	}
}
