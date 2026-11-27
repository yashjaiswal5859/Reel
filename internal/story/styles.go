package story

// StylePreset defines consistent visual directives for image generation (REQ-023).
type StylePreset struct {
	Style           VisualStyle `json:"style"`
	PromptDirective string      `json:"prompt_directive"`
	NegativePrompt  string      `json:"negative_prompt"`
}

var stylePresets = map[VisualStyle]StylePreset{
	Style2DCartoon: {
		Style:           Style2DCartoon,
		PromptDirective: "2D cartoon illustration, clean bold outlines, vibrant cel shading, expressive character design, cohesive color palette",
		NegativePrompt:  "photorealistic, 3d render, blurry, watermark, text, deformed hands, extra limbs",
	},
	Style2DVector: {
		Style:           Style2DVector,
		PromptDirective: "modern 2D vector art, flat geometric shapes, crisp clean edges, smooth gradients, editorial motion graphics style",
		NegativePrompt:  "photorealistic, noisy texture, blurry, watermark, text, sketch",
	},
	StyleStorybook: {
		Style:           StyleStorybook,
		PromptDirective: "whimsical storybook illustration, soft gouache and watercolor texture, warm lighting, rich narrative atmosphere",
		NegativePrompt:  "photorealistic, harsh neon, 3d cgi, watermark, text, deformed",
	},
	StyleAnime: {
		Style:           StyleAnime,
		PromptDirective: "high quality anime key visual, clean lineart, cinematic cel lighting, expressive anime eyes, detailed painted background",
		NegativePrompt:  "western cartoon, photorealistic, blurry, watermark, text, extra fingers",
	},
	Style3DCartoon: {
		Style:           Style3DCartoon,
		PromptDirective: "stylized 3D animated film still, soft global illumination, subsurface scattering, appealing proportions, rich materials",
		NegativePrompt:  "2d flat, low poly, noisy, watermark, text, uncanny valley",
	},
	StyleCinematic: {
		Style:           StyleCinematic,
		PromptDirective: "cinematic digital illustration, dramatic volumetric lighting, rule of thirds composition, atmospheric depth, rich contrast",
		NegativePrompt:  "flat cartoon, low quality, blurry, watermark, text",
	},
	StylePaperCut: {
		Style:           StylePaperCut,
		PromptDirective: "layered paper-cut diorama illustration, tactile craft paper textures, soft drop shadows between depth planes",
		NegativePrompt:  "photorealistic, glossy 3d, blurry, watermark, text",
	},
	StyleMinimalistExplain: {
		Style:           StyleMinimalistExplain,
		PromptDirective: "minimalist explainer illustration, clean uncluttered composition, high-contrast focal subject, modern infographic aesthetic",
		NegativePrompt:  "cluttered background, photorealistic, grunge, watermark, text",
	},
}

// GetStylePreset returns the prompt preset for the requested style (defaults to 2d_cartoon).
func GetStylePreset(style VisualStyle) StylePreset {
	if p, ok := stylePresets[style]; ok {
		return p
	}
	return stylePresets[Style2DCartoon]
}

// SupportedStyles lists all available visual styles.
func SupportedStyles() []VisualStyle {
	return []VisualStyle{
		Style2DCartoon, Style2DVector, StyleStorybook, StyleAnime,
		Style3DCartoon, StyleCinematic, StylePaperCut, StyleMinimalistExplain,
	}
}

// IsValidStyle reports whether s is a supported visual style.
func IsValidStyle(s VisualStyle) bool {
	_, ok := stylePresets[s]
	return ok
}
