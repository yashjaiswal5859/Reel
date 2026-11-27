package composition

// AspectRatio defines target export aspect ratios (REQ-065).
type AspectRatio string

const (
	Aspect9x16 AspectRatio = "9:16"
	Aspect16x9 AspectRatio = "16:9"
	Aspect1x1  AspectRatio = "1:1"
)

// QualityPreset defines target export resolution (REQ-066).
type QualityPreset string

const (
	Quality720p  QualityPreset = "720p"
	Quality1080p QualityPreset = "1080p"
	Quality4K    QualityPreset = "4k"
)

// LayoutMode defines how presenter and story animation panels are arranged (REQ-045).
type LayoutMode string

const (
	LayoutVertical5050   LayoutMode = "vertical_50_50"   // Presenter bottom/top, animation top/bottom
	LayoutVertical4060   LayoutMode = "vertical_40_60"   // 40% presenter, 60% animation
	LayoutHorizontal5050 LayoutMode = "horizontal_50_50" // Left/right 50/50 split
	LayoutHorizontal4060 LayoutMode = "horizontal_40_60" // 40% presenter, 60% animation
)

// ExportConfig controls final split-screen composition and encoding.
type ExportConfig struct {
	AspectRatio     AspectRatio   `json:"aspect_ratio"`
	Quality         QualityPreset `json:"quality"`
	Layout          LayoutMode    `json:"layout"`
	PresenterFirst  bool          `json:"presenter_first"` // true = presenter on top/left; false = animation on top/left
	FPS             int           `json:"fps"`
	CRF             int           `json:"crf"`
}

// DefaultExportConfig returns creator defaults (9:16 vertical 1080p, 50/50 split).
func DefaultExportConfig() ExportConfig {
	return ExportConfig{
		AspectRatio:    Aspect9x16,
		Quality:        Quality1080p,
		Layout:         LayoutVertical5050,
		PresenterFirst: false, // Story animation on top, presenter on bottom
		FPS:            30,
		CRF:            20,
	}
}

// TargetDimensions computes output width & height (even numbers for H.264).
func TargetDimensions(ar AspectRatio, q QualityPreset) (width, height int) {
	shortEdge := 1080
	switch q {
	case Quality720p:
		shortEdge = 720
	case Quality4K:
		shortEdge = 2160
	}

	switch ar {
	case Aspect16x9:
		return (shortEdge * 16) / 9, shortEdge
	case Aspect1x1:
		return shortEdge, shortEdge
	default: // 9:16
		return shortEdge, (shortEdge * 16) / 9
	}
}
