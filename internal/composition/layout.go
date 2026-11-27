package composition

import "fmt"

// PanelRect describes a panel's dimensions and position in the final frame.
type PanelRect struct {
	Width  int
	Height int
}

// SplitLayout holds computed dimensions for both panels and stack direction.
type SplitLayout struct {
	TotalWidth  int
	TotalHeight int
	Horizontal  bool // true = hstack, false = vstack
	Presenter   PanelRect
	Animation   PanelRect
}

// LayoutStrategy computes panel geometry for a given ExportConfig (REQ-045).
type LayoutStrategy interface {
	Name() string
	Compute(cfg ExportConfig) SplitLayout
}

type splitLayoutStrategy struct {
	name           string
	horizontal     bool
	presenterRatio float64
}

// NewLayoutStrategy returns the appropriate SplitScreenLayoutStrategy.
func NewLayoutStrategy(mode LayoutMode, ar AspectRatio) LayoutStrategy {
	switch mode {
	case LayoutVertical4060:
		return splitLayoutStrategy{name: string(mode), horizontal: false, presenterRatio: 0.4}
	case LayoutHorizontal5050:
		return splitLayoutStrategy{name: string(mode), horizontal: true, presenterRatio: 0.5}
	case LayoutHorizontal4060:
		return splitLayoutStrategy{name: string(mode), horizontal: true, presenterRatio: 0.4}
	case LayoutVertical5050:
		return splitLayoutStrategy{name: string(mode), horizontal: false, presenterRatio: 0.5}
	default:
		// Auto-pick horizontal for 16:9, vertical for 9:16 and 1:1.
		return splitLayoutStrategy{name: "auto", horizontal: ar == Aspect16x9, presenterRatio: 0.5}
	}
}

func (s splitLayoutStrategy) Name() string { return s.name }

func (s splitLayoutStrategy) Compute(cfg ExportConfig) SplitLayout {
	w, h := TargetDimensions(cfg.AspectRatio, cfg.Quality)
	if s.horizontal {
		pw := even(int(float64(w) * s.presenterRatio))
		aw := w - pw
		return SplitLayout{
			TotalWidth:  w,
			TotalHeight: h,
			Horizontal:  true,
			Presenter:   PanelRect{Width: pw, Height: h},
			Animation:   PanelRect{Width: aw, Height: h},
		}
	}

	ph := even(int(float64(h) * s.presenterRatio))
	ah := h - ph
	return SplitLayout{
		TotalWidth:  w,
		TotalHeight: h,
		Horizontal:  false,
		Presenter:   PanelRect{Width: w, Height: ph},
		Animation:   PanelRect{Width: w, Height: ah},
	}
}

// BuildSplitFilter constructs the FFmpeg filter_complex string to smart-reframe
// presenter [0:v] and animation [1:v], stack them, and optionally burn ASS subtitles.
func BuildSplitFilter(layout SplitLayout, presenterFirst bool, assFilename string) string {
	presFilter := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1[pres]",
		layout.Presenter.Width, layout.Presenter.Height, layout.Presenter.Width, layout.Presenter.Height)
	animFilter := fmt.Sprintf("[1:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,setsar=1[anim]",
		layout.Animation.Width, layout.Animation.Height, layout.Animation.Width, layout.Animation.Height)

	first, second := "[anim]", "[pres]"
	if presenterFirst {
		first, second = "[pres]", "[anim]"
	}

	stackOp := "vstack=inputs=2"
	if layout.Horizontal {
		stackOp = "hstack=inputs=2"
	}

	if assFilename != "" {
		return fmt.Sprintf("%s;%s;%s%s%s[stacked];[stacked]ass=%s[vout]",
			presFilter, animFilter, first, second, stackOp, assFilename)
	}
	return fmt.Sprintf("%s;%s;%s%s%s[vout]",
		presFilter, animFilter, first, second, stackOp)
}

func even(n int) int {
	if n%2 != 0 {
		return n - 1
	}
	return n
}
