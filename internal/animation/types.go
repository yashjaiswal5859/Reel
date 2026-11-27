package animation

// CharacterAction defines reusable character motion primitives (REQ-032).
type CharacterAction string

const (
	ActionWalk       CharacterAction = "WALK"
	ActionRun        CharacterAction = "RUN"
	ActionJump       CharacterAction = "JUMP"
	ActionTurn       CharacterAction = "TURN"
	ActionSit        CharacterAction = "SIT"
	ActionStand      CharacterAction = "STAND"
	ActionWave       CharacterAction = "WAVE"
	ActionLookAround CharacterAction = "LOOK_AROUND"
	ActionFall       CharacterAction = "FALL"
	ActionPickUp     CharacterAction = "PICK_UP"
	ActionDrop       CharacterAction = "DROP"
	ActionThrow      CharacterAction = "THROW"
	ActionFollow     CharacterAction = "FOLLOW"
	ActionApproach   CharacterAction = "APPROACH"
	ActionMoveAway   CharacterAction = "MOVE_AWAY"
)

// CameraAction defines procedural camera movements (REQ-037).
type CameraAction string

const (
	CamZoomIn   CameraAction = "zoom_in"
	CamZoomOut  CameraAction = "zoom_out"
	CamPanLeft  CameraAction = "pan_left"
	CamPanRight CameraAction = "pan_right"
	CamPanUp    CameraAction = "pan_up"
	CamPanDown  CameraAction = "pan_down"
	CamShake    CameraAction = "shake"
	CamStatic   CameraAction = "static"
)

// EnvironmentEffect defines ambient environment animations (REQ-036).
type EnvironmentEffect string

const (
	EnvNone      EnvironmentEffect = "none"
	EnvWind      EnvironmentEffect = "wind"
	EnvRain      EnvironmentEffect = "rain"
	EnvSnow      EnvironmentEffect = "snow"
	EnvFireGlow  EnvironmentEffect = "fire_glow"
	EnvClouds    EnvironmentEffect = "clouds"
	EnvParticles EnvironmentEffect = "particles"
)

// TransitionType defines scene-to-scene transitions (REQ-039).
type TransitionType string

const (
	TransFade       TransitionType = "fade"
	TransCrossfade  TransitionType = "crossfade"
	TransSlideLeft  TransitionType = "slide_left"
	TransSlideRight TransitionType = "slide_right"
	TransWipe       TransitionType = "wipe"
	TransZoom       TransitionType = "zoom"
	TransCut        TransitionType = "cut"
)

// Event represents a single timed animation instruction inside a scene (REQ-040).
type Event struct {
	Target     string  `json:"target"`      // "character" | "object" | "environment" | "camera"
	Subject    string  `json:"subject"`     // character ID, object name, etc.
	Action     string  `json:"action"`      // e.g. WALK, BOUNCE, SHAKE
	StartSec   float64 `json:"start_sec"`   // relative to scene start
	EndSec     float64 `json:"end_sec"`     // relative to scene start
	Intensity  float64 `json:"intensity"`   // 0.0 - 1.0
}

// ScenePlan is the declarative animation specification for one storyboard scene.
type ScenePlan struct {
	SceneID        string            `json:"scene_id"`
	Index          int               `json:"index"`
	ImageKey       string            `json:"image_key"`
	StartSec       float64           `json:"start_sec"`
	EndSec         float64           `json:"end_sec"`
	DurationSec    float64           `json:"duration_sec"`
	Camera         CameraAction      `json:"camera"`
	Environment    EnvironmentEffect `json:"environment"`
	Transition     TransitionType    `json:"transition"`
	TransitionDur  float64           `json:"transition_dur_sec"`
	Events         []Event           `json:"events"`
}

// Timeline is the complete declarative animation spec for a job (API-first / renderer-agnostic).
type Timeline struct {
	JobID            string      `json:"job_id"`
	Width            int         `json:"width"`
	Height           int         `json:"height"`
	FPS              int         `json:"fps"`
	TotalDurationSec float64     `json:"total_duration_sec"`
	Scenes           []ScenePlan `json:"scenes"`
}
