package animation

import (
	"errors"
	"strings"

	"reel/internal/story"
)

const defaultTransitionDurSec = 0.4

// PlanTimeline converts a directed Storyboard into a synchronized declarative animation Timeline (REQ-040).
func PlanTimeline(jobID string, sb *story.Storyboard, width, height, fps int) (*Timeline, error) {
	if sb == nil || len(sb.Scenes) == 0 {
		return nil, errors.New("cannot plan animation from empty storyboard")
	}
	if width <= 0 {
		width = 1080
	}
	if height <= 0 {
		height = 1080
	}
	if fps <= 0 {
		fps = 30
	}

	plans := make([]ScenePlan, 0, len(sb.Scenes))
	var maxEnd float64

	for i, sc := range sb.Scenes {
		dur := sc.EndSec - sc.StartSec
		if dur <= 0 {
			dur = 2.0
		}

		cam := normalizeCamera(sc.CameraMotion, i)
		env := inferEnvironment(sc.Location, sc.Description)
		trans := normalizeTransition(sc.Transition)

		transDur := defaultTransitionDurSec
		if dur <= transDur*2 {
			transDur = dur * 0.25
		}

		events := make([]Event, 0, len(sc.Actions)+len(sc.Objects)+2)
		events = append(events, Event{
			Target:    "camera",
			Subject:   "main_camera",
			Action:    string(cam),
			StartSec:  0,
			EndSec:    dur,
			Intensity: 0.5,
		})

		if env != EnvNone {
			events = append(events, Event{
				Target:    "environment",
				Subject:   sc.Location,
				Action:    string(env),
				StartSec:  0,
				EndSec:    dur,
				Intensity: 0.4,
			})
		}

		for _, act := range sc.Actions {
			subj := "character"
			if len(sc.CharacterIDs) > 0 {
				subj = sc.CharacterIDs[0]
			}
			events = append(events, Event{
				Target:    "character",
				Subject:   subj,
				Action:    strings.ToUpper(strings.TrimSpace(act)),
				StartSec:  0,
				EndSec:    dur,
				Intensity: 0.7,
			})
		}

		for _, obj := range sc.Objects {
			events = append(events, Event{
				Target:    "object",
				Subject:   obj,
				Action:    "OBJECT_MOTION",
				StartSec:  0,
				EndSec:    dur,
				Intensity: 0.5,
			})
		}

		plans = append(plans, ScenePlan{
			SceneID:       sc.SceneID,
			Index:         sc.Index,
			ImageKey:      sc.ImageKey,
			StartSec:      sc.StartSec,
			EndSec:        sc.EndSec,
			DurationSec:   dur,
			Camera:        cam,
			Environment:   env,
			Transition:    trans,
			TransitionDur: transDur,
			Events:        events,
		})

		if sc.EndSec > maxEnd {
			maxEnd = sc.EndSec
		}
	}

	return &Timeline{
		JobID:            jobID,
		Width:            width,
		Height:           height,
		FPS:              fps,
		TotalDurationSec: maxEnd,
		Scenes:           plans,
	}, nil
}

func normalizeCamera(raw string, idx int) CameraAction {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "zoom_in":
		return CamZoomIn
	case "zoom_out":
		return CamZoomOut
	case "pan_left":
		return CamPanLeft
	case "pan_right":
		return CamPanRight
	case "pan_up":
		return CamPanUp
	case "pan_down":
		return CamPanDown
	case "shake":
		return CamShake
	case "static":
		return CamStatic
	default:
		// Alternate subtle motion so scenes never feel dead.
		cycle := []CameraAction{CamZoomIn, CamPanRight, CamZoomOut, CamPanLeft}
		return cycle[idx%len(cycle)]
	}
}

func inferEnvironment(location, desc string) EnvironmentEffect {
	combined := strings.ToLower(location + " " + desc)
	switch {
	case strings.Contains(combined, "rain") || strings.Contains(combined, "storm"):
		return EnvRain
	case strings.Contains(combined, "snow") || strings.Contains(combined, "winter"):
		return EnvSnow
	case strings.Contains(combined, "fire") || strings.Contains(combined, "torch") || strings.Contains(combined, "flame"):
		return EnvFireGlow
	case strings.Contains(combined, "forest") || strings.Contains(combined, "tree") || strings.Contains(combined, "wind"):
		return EnvWind
	case strings.Contains(combined, "sky") || strings.Contains(combined, "cloud"):
		return EnvClouds
	case strings.Contains(combined, "magic") || strings.Contains(combined, "glow"):
		return EnvParticles
	default:
		return EnvNone
	}
}

func normalizeTransition(raw string) TransitionType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "crossfade":
		return TransCrossfade
	case "slide", "slide_left":
		return TransSlideLeft
	case "slide_right":
		return TransSlideRight
	case "wipe":
		return TransWipe
	case "zoom":
		return TransZoom
	case "cut":
		return TransCut
	default:
		return TransFade
	}
}
