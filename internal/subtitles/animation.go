package subtitles

import (
	"fmt"
	"strings"
)

// AnimationStrategy formats ASS override tags for a given caption animation style (REQ-011).
type AnimationStrategy interface {
	Name() string
	PrefixTags(cue Cue, place PlacementRegion) string
	FormatWords(cue Cue) string
}

var animationRegistry = map[string]AnimationStrategy{
	"pop":        popAnimation{},
	"bounce":     bounceAnimation{},
	"fade":       fadeAnimation{},
	"slide":      slideAnimation{},
	"scale":      scaleAnimation{},
	"typewriter": typewriterAnimation{},
	"kinetic":    kineticAnimation{},
	"karaoke":    karaokeAnimation{},
}

// NewAnimationStrategy returns the requested caption animation Strategy (defaults to "pop").
func NewAnimationStrategy(template string) AnimationStrategy {
	key := strings.ToLower(strings.TrimSpace(template))
	if s, ok := animationRegistry[key]; ok {
		return s
	}
	return animationRegistry["pop"]
}

// SupportedTemplates lists all available animated caption template names.
func SupportedTemplates() []string {
	return []string{"pop", "bounce", "fade", "slide", "scale", "typewriter", "kinetic", "karaoke"}
}

func formatKaraokeWords(cue Cue) string {
	if len(cue.Words) == 0 {
		return escapeASS(cue.Text)
	}
	var sb strings.Builder
	for i, w := range cue.Words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		cs := w.DurationCS
		if cs < 1 {
			cs = 1
		}
		fmt.Fprintf(&sb, `{\kf%d}%s`, cs, escapeASS(w.Text))
	}
	return sb.String()
}

type popAnimation struct{}

func (popAnimation) Name() string { return "pop" }
func (popAnimation) PrefixTags(_ Cue, _ PlacementRegion) string {
	return `{\fscx80\fscy80\t(0,90,\fscx108\fscy108)\t(90,160,\fscx100\fscy100)}`
}
func (popAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }

type bounceAnimation struct{}

func (bounceAnimation) Name() string { return "bounce" }
func (bounceAnimation) PrefixTags(_ Cue, _ PlacementRegion) string {
	return `{\fscx70\fscy120\t(0,110,\fscx110\fscy92)\t(110,200,\fscx100\fscy100)}`
}
func (bounceAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }

type fadeAnimation struct{}

func (fadeAnimation) Name() string { return "fade" }
func (fadeAnimation) PrefixTags(_ Cue, _ PlacementRegion) string {
	return `{\fad(120,120)}`
}
func (fadeAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }

type slideAnimation struct{}

func (slideAnimation) Name() string { return "slide" }
func (slideAnimation) PrefixTags(_ Cue, place PlacementRegion) string {
	x := place.PlayResX / 2
	yEnd := place.PlayResY - place.MarginV
	if place.ASSAlign == 8 {
		yEnd = place.MarginV
	} else if place.ASSAlign == 5 {
		yEnd = place.PlayResY / 2
	}
	yStart := yEnd + 40
	return fmt.Sprintf(`{\move(%d,%d,%d,%d,0,160)\fad(100,80)}`, x, yStart, x, yEnd)
}
func (slideAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }

type scaleAnimation struct{}

func (scaleAnimation) Name() string { return "scale" }
func (scaleAnimation) PrefixTags(_ Cue, _ PlacementRegion) string {
	return `{\fscx60\fscy60\t(0,180,\fscx100\fscy100)}`
}
func (scaleAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }

type typewriterAnimation struct{}

func (typewriterAnimation) Name() string { return "typewriter" }
func (typewriterAnimation) PrefixTags(_ Cue, _ PlacementRegion) string {
	return ""
}
func (typewriterAnimation) FormatWords(cue Cue) string {
	if len(cue.Words) == 0 {
		return escapeASS(cue.Text)
	}
	var sb strings.Builder
	elapsedMS := 0
	for i, w := range cue.Words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		durMS := w.DurationCS * 10
		if durMS < 40 {
			durMS = 40
		}
		fmt.Fprintf(&sb, `{\alpha&HFF&\t(%d,%d,\alpha&H00&)}%s`, elapsedMS, elapsedMS+40, escapeASS(w.Text))
		elapsedMS += durMS
	}
	return sb.String()
}

type kineticAnimation struct{}

func (kineticAnimation) Name() string { return "kinetic" }
func (kineticAnimation) PrefixTags(cue Cue, _ PlacementRegion) string {
	angle := 2
	if cue.Index%2 == 0 {
		angle = -2
	}
	return fmt.Sprintf(`{\frz%d\fscx85\fscy85\t(0,140,\frz0\fscx102\fscy102)}`, angle)
}
func (kineticAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }

type karaokeAnimation struct{}

func (karaokeAnimation) Name() string { return "karaoke" }
func (karaokeAnimation) PrefixTags(_ Cue, _ PlacementRegion) string {
	return ""
}
func (karaokeAnimation) FormatWords(cue Cue) string { return formatKaraokeWords(cue) }
