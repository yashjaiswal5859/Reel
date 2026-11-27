package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"reel/internal/ai/imagegen"
	"reel/internal/storage"
	"reel/internal/story"
	"reel/internal/temporal/activities"
)

type storyboardHandler struct {
	storage          storage.Strategy
	imageGen         imagegen.Strategy
	maxRegenAttempts int
}

// SceneCard is the viewer representation of a storyboard scene with derived preview URLs.
type SceneCard struct {
	story.Scene
	DurationSec float64 `json:"duration_sec"`
	PreviewURL  string  `json:"preview_url,omitempty"`
}

// ScenePatch is a declarative JSON patch for agent-native or UI scene editing.
type ScenePatch struct {
	Description  *string           `json:"description,omitempty"`
	StartSec     *float64          `json:"start_sec,omitempty"`
	EndSec       *float64          `json:"end_sec,omitempty"`
	DurationSec  *float64          `json:"duration_sec,omitempty"`
	VisualType   *story.VisualType `json:"visual_type,omitempty"`
	Location     *string           `json:"location,omitempty"`
	Actions      *[]string         `json:"actions,omitempty"`
	Objects      *[]string         `json:"objects,omitempty"`
	CharacterIDs *[]string         `json:"character_ids,omitempty"`
	CameraMotion *string           `json:"camera_motion,omitempty"`
	Transition   *string           `json:"transition,omitempty"`
	Prompt       *string           `json:"prompt,omitempty"`
}

// RegenerateSceneRequest holds optional overrides when regenerating a single scene illustration.
type RegenerateSceneRequest struct {
	Prompt string `json:"prompt,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// UpdateStyleRequest sets a job's storyboard visual style.
type UpdateStyleRequest struct {
	Style story.VisualStyle `json:"style" binding:"required"`
}

func (h *storyboardHandler) get(c *gin.Context) {
	doc, ok := h.loadStoryboard(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *storyboardHandler) listScenes(c *gin.Context) {
	doc, ok := h.loadStoryboard(c)
	if !ok {
		return
	}
	if doc.Storyboard == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "storyboard has no content"})
		return
	}

	jobID := c.Param("id")
	cards := make([]SceneCard, len(doc.Storyboard.Scenes))
	for i, sc := range doc.Storyboard.Scenes {
		cards[i] = toSceneCard(jobID, sc)
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id":  jobID,
		"style":   doc.Storyboard.Style,
		"title":   doc.Storyboard.Analysis.Title,
		"summary": doc.Storyboard.Analysis.Summary,
		"quality": doc.Quality,
		"scenes":  cards,
	})
}

func (h *storyboardHandler) getScene(c *gin.Context) {
	doc, ok := h.loadStoryboard(c)
	if !ok {
		return
	}
	jobID := c.Param("id")
	sceneID := c.Param("sceneId")
	idx := findSceneIndex(doc, sceneID)
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("scene %q not found", sceneID)})
		return
	}
	c.JSON(http.StatusOK, toSceneCard(jobID, doc.Storyboard.Scenes[idx]))
}

func (h *storyboardHandler) patchScene(c *gin.Context) {
	doc, ok := h.loadStoryboard(c)
	if !ok {
		return
	}
	jobID := c.Param("id")
	sceneID := c.Param("sceneId")
	idx := findSceneIndex(doc, sceneID)
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("scene %q not found", sceneID)})
		return
	}

	var patch ScenePatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid scene patch: %v", err)})
		return
	}

	sc := doc.Storyboard.Scenes[idx]
	if patch.Description != nil {
		sc.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.StartSec != nil {
		sc.StartSec = *patch.StartSec
	}
	if patch.EndSec != nil {
		sc.EndSec = *patch.EndSec
	} else if patch.DurationSec != nil {
		sc.EndSec = sc.StartSec + *patch.DurationSec
	}
	if sc.StartSec < 0 || sc.EndSec <= sc.StartSec {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scene timing: require 0 <= start_sec < end_sec"})
		return
	}
	if patch.VisualType != nil {
		if !story.IsValidVisualType(*patch.VisualType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported visual_type %q", *patch.VisualType)})
			return
		}
		sc.VisualType = *patch.VisualType
	}
	if patch.Location != nil {
		sc.Location = strings.TrimSpace(*patch.Location)
	}
	if patch.Actions != nil {
		sc.Actions = *patch.Actions
	}
	if patch.Objects != nil {
		sc.Objects = *patch.Objects
	}
	if patch.CharacterIDs != nil {
		sc.CharacterIDs = *patch.CharacterIDs
	}
	if patch.CameraMotion != nil {
		sc.CameraMotion = strings.TrimSpace(*patch.CameraMotion)
	}
	if patch.Transition != nil {
		sc.Transition = strings.TrimSpace(*patch.Transition)
	}
	if patch.Prompt != nil {
		sc.Prompt = strings.TrimSpace(*patch.Prompt)
	}

	doc.Storyboard.Scenes[idx] = sc
	totalDur := maxStoryboardEnd(doc.Storyboard.Scenes)
	doc.Quality = story.ValidateStoryboard(doc.Storyboard, totalDur)

	if err := h.saveStoryboard(c.Request.Context(), jobID, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save storyboard: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scene":   toSceneCard(jobID, sc),
		"quality": doc.Quality,
	})
}

func (h *storyboardHandler) regenerateScene(c *gin.Context) {
	if h.imageGen == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image generation provider not configured"})
		return
	}

	doc, ok := h.loadStoryboard(c)
	if !ok {
		return
	}
	jobID := c.Param("id")
	sceneID := c.Param("sceneId")
	idx := findSceneIndex(doc, sceneID)
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("scene %q not found", sceneID)})
		return
	}

	sc := doc.Storyboard.Scenes[idx]
	if h.maxRegenAttempts > 0 && sc.RegenCount >= h.maxRegenAttempts {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": fmt.Sprintf("scene %q reached max regeneration attempts (%d)", sceneID, h.maxRegenAttempts),
		})
		return
	}

	var req RegenerateSceneRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid regenerate payload: %v", err)})
			return
		}
	}

	w, ht := req.Width, req.Height
	if w <= 0 || ht <= 0 {
		w, ht = 1024, 1024
	}

	prompt, neg := story.BuildScenePrompt(sc, doc.Storyboard.CharacterBible, doc.Storyboard.Style)
	if strings.TrimSpace(req.Prompt) != "" {
		prompt = strings.TrimSpace(req.Prompt)
	}

	var refs []string
	for _, cid := range sc.CharacterIDs {
		for _, ch := range doc.Storyboard.CharacterBible {
			if ch.ID == cid && ch.ReferenceImageKey != "" {
				if u, err := h.storage.URL(c.Request.Context(), ch.ReferenceImageKey, 30*time.Minute); err == nil && strings.HasPrefix(u, "http") {
					refs = append(refs, u)
				}
			}
		}
	}

	resp, err := h.imageGen.Generate(c.Request.Context(), imagegen.GenerateRequest{
		Prompt:         prompt,
		NegativePrompt: neg,
		Width:          w,
		Height:         ht,
		ReferenceURLs:  refs,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("image generation failed: %v", err)})
		return
	}

	key := fmt.Sprintf("jobs/%s/scenes/%s.png", jobID, sc.SceneID)
	ct := resp.ContentType
	if ct == "" {
		ct = "image/png"
	}
	if err := h.storage.Put(c.Request.Context(), key, bytes.NewReader(resp.ImageBytes), int64(len(resp.ImageBytes)), ct); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("store regenerated image: %v", err)})
		return
	}

	sc.Prompt = prompt
	sc.ImageKey = key
	sc.RegenCount++
	doc.Storyboard.Scenes[idx] = sc

	if err := h.saveStoryboard(c.Request.Context(), jobID, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save storyboard: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scene": toSceneCard(jobID, sc),
	})
}

func (h *storyboardHandler) updateStyle(c *gin.Context) {
	doc, ok := h.loadStoryboard(c)
	if !ok {
		return
	}

	var req UpdateStyleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid style payload: %v", err)})
		return
	}
	if !story.IsValidStyle(req.Style) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":            fmt.Sprintf("unsupported visual style %q", req.Style),
			"supported_styles": story.SupportedStyles(),
		})
		return
	}

	doc.Storyboard.Style = req.Style
	jobID := c.Param("id")
	if err := h.saveStoryboard(c.Request.Context(), jobID, doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save storyboard: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id": jobID,
		"style":  req.Style,
		"preset": story.GetStylePreset(req.Style),
	})
}

func (h *storyboardHandler) loadStoryboard(c *gin.Context) (*activities.StoredStoryboard, bool) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return nil, false
	}

	jobID := c.Param("id")
	key := fmt.Sprintf("jobs/%s/storyboard.json", jobID)

	rc, err := h.storage.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("storyboard for job %q not found", jobID)})
		return nil, false
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read storyboard"})
		return nil, false
	}

	var doc activities.StoredStoryboard
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt storyboard data"})
		return nil, false
	}
	return &doc, true
}

func (h *storyboardHandler) saveStoryboard(ctx context.Context, jobID string, doc *activities.StoredStoryboard) error {
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("jobs/%s/storyboard.json", jobID)
	return h.storage.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "application/json")
}

func findSceneIndex(doc *activities.StoredStoryboard, sceneID string) int {
	if doc == nil || doc.Storyboard == nil {
		return -1
	}
	for i, sc := range doc.Storyboard.Scenes {
		if sc.SceneID == sceneID {
			return i
		}
	}
	return -1
}

func toSceneCard(jobID string, sc story.Scene) SceneCard {
	card := SceneCard{
		Scene:       sc,
		DurationSec: sc.EndSec - sc.StartSec,
	}
	if sc.ImageKey != "" {
		prefix := fmt.Sprintf("jobs/%s/", jobID)
		rel := strings.TrimPrefix(sc.ImageKey, prefix)
		card.PreviewURL = fmt.Sprintf("/api/v1/jobs/%s/assets/%s", jobID, rel)
	}
	return card
}

func maxStoryboardEnd(scenes []story.Scene) float64 {
	var maxEnd float64
	for _, sc := range scenes {
		if sc.EndSec > maxEnd {
			maxEnd = sc.EndSec
		}
	}
	return maxEnd
}
