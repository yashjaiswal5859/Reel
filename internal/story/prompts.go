package story

import (
	"fmt"
	"strings"
)

// BuildCharacterSheetPrompt creates a consistent character reference sheet prompt (REQ-026).
func BuildCharacterSheetPrompt(char CharacterProfile, style VisualStyle) (prompt, negative string) {
	preset := GetStylePreset(style)
	var parts []string
	parts = append(parts, preset.PromptDirective)
	parts = append(parts, fmt.Sprintf("character reference sheet of %s, front and three-quarter view on clean neutral background", char.Name))

	if char.PhysicalAppearance != "" {
		parts = append(parts, char.PhysicalAppearance)
	}
	if char.FaceAndHairstyle != "" {
		parts = append(parts, "face and hair: "+char.FaceAndHairstyle)
	}
	if char.Clothing != "" {
		parts = append(parts, "wearing "+char.Clothing)
	}
	if char.BodyProportions != "" {
		parts = append(parts, "proportions: "+char.BodyProportions)
	}
	if len(char.Colors) > 0 {
		parts = append(parts, "signature colors: "+strings.Join(char.Colors, ", "))
	}
	if char.DistinguishingFeature != "" {
		parts = append(parts, "distinguishing features: "+char.DistinguishingFeature)
	}

	return strings.Join(parts, ", "), preset.NegativePrompt
}

// BuildScenePrompt creates a detailed scene illustration prompt incorporating style,
// Character Bible details, environment, objects, camera composition, and continuity (REQ-024).
func BuildScenePrompt(scene Scene, bible []CharacterProfile, style VisualStyle) (prompt, negative string) {
	preset := GetStylePreset(style)
	charMap := make(map[string]CharacterProfile, len(bible))
	for _, c := range bible {
		charMap[c.ID] = c
	}

	var parts []string
	parts = append(parts, preset.PromptDirective)

	if scene.Description != "" {
		parts = append(parts, scene.Description)
	}
	if scene.Location != "" {
		parts = append(parts, "setting: "+scene.Location)
	}

	for _, cid := range scene.CharacterIDs {
		if c, ok := charMap[cid]; ok {
			desc := fmt.Sprintf("%s (%s, %s, wearing %s)", c.Name, c.PhysicalAppearance, c.FaceAndHairstyle, c.Clothing)
			parts = append(parts, "character: "+desc)
		}
	}

	if len(scene.Objects) > 0 {
		parts = append(parts, "objects: "+strings.Join(scene.Objects, ", "))
	}
	if len(scene.Continuity.ObjectOwnership) > 0 {
		var owned []string
		for obj, ownerID := range scene.Continuity.ObjectOwnership {
			ownerName := ownerID
			if c, ok := charMap[ownerID]; ok {
				ownerName = c.Name
			}
			owned = append(owned, fmt.Sprintf("%s holding %s", ownerName, obj))
		}
		parts = append(parts, "continuity: "+strings.Join(owned, "; "))
	}
	if len(scene.Actions) > 0 {
		parts = append(parts, "action: "+strings.Join(scene.Actions, ", "))
	}
	if scene.CameraMotion != "" {
		parts = append(parts, "composition suitable for "+scene.CameraMotion)
	}

	return strings.Join(parts, ", "), preset.NegativePrompt
}
