package subtitles

import "reel/internal/media"

// PlacementRegion defines computed pixel coordinates and ASS alignment for captions (REQ-048).
type PlacementRegion struct {
	PlayResX   int `json:"play_res_x"`
	PlayResY   int `json:"play_res_y"`
	ASSAlign   int `json:"ass_align"` // ASS numpad alignment: 2=bottom-center, 5=mid-center, 8=top-center
	MarginL    int `json:"margin_l"`
	MarginR    int `json:"margin_r"`
	MarginV    int `json:"margin_v"`
	FontSizePx int `json:"font_size_px"`
}

// ComputePlacement calculates safe caption margins and font scaling so captions
// avoid overlapping speaker faces or split-screen edges across 9:16, 16:9, and 1:1.
func ComputePlacement(meta *media.VideoMetadata, style StyleConfig) PlacementRegion {
	w, h := 1080, 1920
	if meta != nil && meta.Width > 0 && meta.Height > 0 {
		w = meta.Width
		h = meta.Height
	}

	assAlign := 2 // bottom-center
	switch style.Alignment {
	case AlignCenter:
		assAlign = 5
	case AlignTop:
		assAlign = 8
	}

	marginPct := style.MarginBottomPct
	if marginPct <= 0 || marginPct > 45 {
		marginPct = 12.0
		if meta != nil && meta.Orientation == "portrait" {
			// Portrait mobile videos need extra bottom clearance for platform UI & split-screen seam.
			marginPct = 15.0
		}
	}

	marginH := int(float64(w) * 0.08)
	marginV := int(float64(h) * (marginPct / 100.0))

	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = 56
	}
	// Scale font proportionally to 1080p reference width.
	scaledFont := int(float64(fontSize) * (float64(w) / 1080.0))
	if scaledFont < 24 {
		scaledFont = 24
	}

	return PlacementRegion{
		PlayResX:   w,
		PlayResY:   h,
		ASSAlign:   assAlign,
		MarginL:    marginH,
		MarginR:    marginH,
		MarginV:    marginV,
		FontSizePx: scaledFont,
	}
}
