package story

// VisualStyle represents the project-wide art style (REQ-023).
type VisualStyle string

const (
	Style2DCartoon          VisualStyle = "2d_cartoon"
	Style2DVector           VisualStyle = "2d_vector"
	StyleStorybook          VisualStyle = "storybook"
	StyleAnime              VisualStyle = "anime"
	Style3DCartoon          VisualStyle = "3d_cartoon"
	StyleCinematic          VisualStyle = "cinematic"
	StylePaperCut           VisualStyle = "paper_cut"
	StyleMinimalistExplain  VisualStyle = "minimalist_explainer"
)

// VisualType represents the AI Director's visual choice for a scene (REQ-022).
type VisualType string

const (
	VisualCharacterAnimation VisualType = "character_animation"
	VisualEnvironment        VisualType = "environment_illustration"
	VisualObjectAnimation    VisualType = "object_animation"
	VisualAnimatedDiagram    VisualType = "animated_diagram"
	VisualChartInfographic   VisualType = "chart_infographic"
	VisualKineticTypography  VisualType = "kinetic_typography"
	VisualMetaphor           VisualType = "visual_metaphor"
	VisualStaticMotion       VisualType = "static_motion"
)

// CharacterProfile is the persistent Character Bible entry (REQ-018).
type CharacterProfile struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	PhysicalAppearance    string   `json:"physical_appearance"`
	FaceAndHairstyle      string   `json:"face_and_hairstyle"`
	Clothing              string   `json:"clothing"`
	BodyProportions       string   `json:"body_proportions"`
	Colors                []string `json:"colors"`
	DistinguishingFeature string   `json:"distinguishing_features"`
	ReferenceImageKey     string   `json:"reference_image_key,omitempty"`
}

// StoryAnalysis captures full-transcript story understanding before scene generation (REQ-016).
type StoryAnalysis struct {
	Title          string             `json:"title"`
	Summary        string             `json:"summary"`
	Tone           string             `json:"tone"`
	Characters     []CharacterProfile `json:"characters"`
	Locations      []string           `json:"locations"`
	KeyObjects     []string           `json:"key_objects"`
	Chronology     []string           `json:"chronological_events"`
	Relationships  []string           `json:"relationships"`
}

// NarrativeState tracks continuity across scenes (REQ-021).
type NarrativeState struct {
	CharacterPositions map[string]string `json:"character_positions,omitempty"`
	ObjectOwnership    map[string]string `json:"object_ownership,omitempty"`
	ObjectStates       map[string]string `json:"object_states,omitempty"`
	CompletedActions   []string          `json:"completed_actions,omitempty"`
}

// Scene represents a single visual scene synchronized to the narration (REQ-017).
type Scene struct {
	SceneID        string         `json:"scene_id"`
	Index          int            `json:"index"`
	StartSec       float64        `json:"start_sec"`
	EndSec         float64        `json:"end_sec"`
	TranscriptText string         `json:"transcript_text"`
	VisualType     VisualType     `json:"visual_type"`
	Description    string         `json:"description"`
	Location       string         `json:"location"`
	CharacterIDs   []string       `json:"character_ids"`
	Objects        []string       `json:"objects"`
	Actions        []string       `json:"actions"`
	CameraMotion   string         `json:"camera_motion"`
	Transition     string         `json:"transition"`
	Continuity     NarrativeState `json:"continuity_state"`
	Prompt         string         `json:"prompt,omitempty"`
	ImageKey       string         `json:"image_key,omitempty"`
	RegenCount     int            `json:"regen_count,omitempty"`
}

// Storyboard is the complete directed visual plan for a video.
type Storyboard struct {
	Style          VisualStyle        `json:"style"`
	Analysis       StoryAnalysis      `json:"analysis"`
	CharacterBible []CharacterProfile `json:"character_bible"`
	Scenes         []Scene            `json:"scenes"`
}

var validVisualTypes = map[VisualType]bool{
	VisualCharacterAnimation: true,
	VisualEnvironment:        true,
	VisualObjectAnimation:    true,
	VisualAnimatedDiagram:    true,
	VisualChartInfographic:   true,
	VisualKineticTypography:  true,
	VisualMetaphor:           true,
	VisualStaticMotion:       true,
}

// IsValidVisualType checks if vt is a supported scene visual type.
func IsValidVisualType(vt VisualType) bool {
	return validVisualTypes[vt]
}
