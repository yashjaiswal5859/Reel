package subtitles

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"reel/internal/media"
)

// RenderASS streams an Advanced SubStation Alpha (.ass) subtitle file to w
// with karaoke word-highlight timings and animation tags.
func RenderASS(w io.Writer, track *Track, meta *media.VideoMetadata) error {
	if track == nil {
		return fmt.Errorf("subtitle track is nil")
	}

	bw := bufio.NewWriterSize(w, 16*1024)
	place := ComputePlacement(meta, track.Style)
	anim := NewAnimationStrategy(track.Style.Template)

	boldFlag := 0
	if strings.EqualFold(track.Style.FontWeight, "bold") {
		boldFlag = -1
	}

	fontFamily := track.Style.FontFamily
	if fontFamily == "" {
		fontFamily = "Arial"
	}

	primaryColor := hexToASSColor(track.Style.HighlightColor, "&H0000D7FF")
	secondaryColor := hexToASSColor(track.Style.TextColor, "&H00FFFFFF")
	outlineColor := hexToASSColor(track.Style.OutlineColor, "&H00000000")
	backColor := hexToASSColor(track.Style.BackgroundColor, "&H80000000")

	if _, err := fmt.Fprintf(bw, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 0\nScaledBorderAndShadow: yes\n\n",
		place.PlayResX, place.PlayResY); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(bw, "[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(bw, "Style: Default,%s,%d,%s,%s,%s,%s,%d,0,0,0,100,100,0,0,1,%.1f,%.1f,%d,%d,%d,%d,1\n\n",
		fontFamily, place.FontSizePx,
		primaryColor, secondaryColor, outlineColor, backColor,
		boldFlag, track.Style.OutlineWidth, track.Style.ShadowDepth,
		place.ASSAlign, place.MarginL, place.MarginR, place.MarginV,
	); err != nil {
		return err
	}

	if _, err := fmt.Fprint(bw, "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"); err != nil {
		return err
	}

	for _, cue := range track.Cues {
		startTS := formatASSTime(cue.Start)
		endTS := formatASSTime(cue.End)
		tags := anim.PrefixTags(cue, place)
		words := anim.FormatWords(cue)

		if _, err := fmt.Fprintf(bw, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s%s\n",
			startTS, endTS, tags, words); err != nil {
			return err
		}
	}

	return bw.Flush()
}

func formatASSTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	totalCS := int(sec*100 + 0.5)
	cs := totalCS % 100
	totalSec := totalCS / 100
	s := totalSec % 60
	totalMin := totalSec / 60
	m := totalMin % 60
	h := totalMin / 60
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

// hexToASSColor converts "#RRGGBB" or "#RRGGBBAA" to ASS "&HAABBGGRR".
func hexToASSColor(hex, fallback string) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) == 6 {
		return strings.ToUpper("&H00" + h[4:6] + h[2:4] + h[0:2])
	}
	if len(h) == 8 {
		return strings.ToUpper("&H" + h[6:8] + h[4:6] + h[2:4] + h[0:2])
	}
	return fallback
}

func escapeASS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "{", `\{`)
	s = strings.ReplaceAll(s, "}", `\}`)
	return s
}
