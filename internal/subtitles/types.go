package subtitles

// Alignment defines caption anchor position on screen.
type Alignment string

const (
	AlignBottom Alignment = "bottom"
	AlignCenter Alignment = "center"
	AlignTop    Alignment = "top"
)

// StyleConfig holds user-customizable visual presentation settings (REQ-012).
type StyleConfig struct {
	Template        string    `json:"template"`          // "pop" | "bounce" | "fade" | "slide" | "scale" | "typewriter" | "kinetic" | "karaoke"
	FontFamily      string    `json:"font_family"`
	FontSize        int       `json:"font_size"`
	FontWeight      string    `json:"font_weight"`       // "normal" | "bold"
	TextColor       string    `json:"text_color"`        // hex "#FFFFFF"
	HighlightColor  string    `json:"highlight_color"`   // hex "#FFD700"
	BackgroundColor string    `json:"background_color"`  // hex "#00000080" (optional)
	OutlineColor    string    `json:"outline_color"`     // hex "#000000"
	OutlineWidth    float64   `json:"outline_width"`
	ShadowDepth     float64   `json:"shadow_depth"`
	Alignment       Alignment `json:"alignment"`         // "bottom" | "center" | "top"
	MaxWordsPerLine int       `json:"max_words_per_line"`
	MarginBottomPct float64   `json:"margin_bottom_pct"` // safe margin percentage
}

// DefaultStyle returns sensible creator defaults for mobile-friendly captions.
func DefaultStyle() StyleConfig {
	return StyleConfig{
		Template:        "pop",
		FontFamily:      "Arial",
		FontSize:        56,
		FontWeight:      "bold",
		TextColor:       "#FFFFFF",
		HighlightColor:  "#FFD700",
		BackgroundColor: "#00000080",
		OutlineColor:    "#000000",
		OutlineWidth:    3.0,
		ShadowDepth:     1.5,
		Alignment:       AlignBottom,
		MaxWordsPerLine: 4,
		MarginBottomPct: 12.0,
	}
}

// WordCue represents a single word's timing and karaoke highlight window.
type WordCue struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	DurationCS int     `json:"duration_cs"` // centiseconds for ASS \kf
}

// Cue represents one displayed caption line/block.
type Cue struct {
	Index int       `json:"index"`
	Text  string    `json:"text"`
	Start float64   `json:"start"`
	End   float64   `json:"end"`
	Words []WordCue `json:"words"`
}

// Track is the complete generated subtitle artifact for a job.
type Track struct {
	Style StyleConfig `json:"style"`
	Cues  []Cue       `json:"cues"`
}
