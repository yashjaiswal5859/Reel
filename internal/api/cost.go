package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"reel/internal/cost"
	"reel/internal/storage"
	"reel/internal/temporal/activities"
)

type costHandler struct {
	storage          storage.Strategy
	jobs             JobService
	defaultASR       string
	defaultLLM       string
	defaultImageGen  string
}

func (h *costHandler) estimate(c *gin.Context) {
	var req cost.EstimateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid estimate request: %v", err)})
		return
	}
	if req.DurationSec <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "duration_sec must be greater than 0"})
		return
	}
	if req.ASRProvider == "" {
		req.ASRProvider = h.defaultASR
	}
	if req.LLMProvider == "" {
		req.LLMProvider = h.defaultLLM
	}
	if req.ImageGenProvider == "" {
		req.ImageGenProvider = h.defaultImageGen
	}

	c.JSON(http.StatusOK, cost.Estimate(req))
}

func (h *costHandler) getJobCost(c *gin.Context) {
	jobID := c.Param("id")
	req := cost.EstimateRequest{
		ASRProvider:      h.defaultASR,
		LLMProvider:      h.defaultLLM,
		ImageGenProvider: h.defaultImageGen,
		Quality:          c.Query("quality"),
	}

	foundAny := false
	hasActualStoryboard := false

	if h.jobs != nil {
		if status, err := h.jobs.GetJobStatus(c.Request.Context(), jobID); err == nil && status != nil {
			foundAny = true
			if status.Metadata != nil && status.Metadata.DurationSec > 0 {
				req.DurationSec = status.Metadata.DurationSec
			}
		}
	}

	if h.storage != nil {
		if sb, ok := h.readStoryboard(c, jobID); ok {
			foundAny = true
			hasActualStoryboard = true
			if sb.Provider != "" {
				req.LLMProvider = sb.Provider
			}
			if sb.Usage.TotalTokens > 0 {
				req.LLMTokensUsed = sb.Usage.TotalTokens
			}
			if sb.Storyboard != nil {
				req.SceneCount = len(sb.Storyboard.Scenes)
				req.CharacterCount = len(sb.Storyboard.CharacterBible)
				for _, sc := range sb.Storyboard.Scenes {
					if sc.EndSec > req.DurationSec {
						req.DurationSec = sc.EndSec
					}
				}
			}
		}

		if req.DurationSec <= 0 {
			if tr, ok := h.readTranscript(c, jobID); ok {
				foundAny = true
				if tr.Transcript != nil {
					if tr.Transcript.Provider != "" {
						req.ASRProvider = tr.Transcript.Provider
					}
					if tr.Transcript.Duration > 0 {
						req.DurationSec = tr.Transcript.Duration
					}
				}
			}
		}
	}

	if !foundAny {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("job %q not found", jobID)})
		return
	}

	est := cost.Estimate(req)
	c.JSON(http.StatusOK, gin.H{
		"job_id":    jobID,
		"is_actual": hasActualStoryboard,
		"estimate":  est,
	})
}

func (h *costHandler) readStoryboard(c *gin.Context, jobID string) (*activities.StoredStoryboard, bool) {
	rc, err := h.storage.Get(c.Request.Context(), fmt.Sprintf("jobs/%s/storyboard.json", jobID))
	if err != nil {
		return nil, false
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		return nil, false
	}
	var doc activities.StoredStoryboard
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, false
	}
	return &doc, true
}

func (h *costHandler) readTranscript(c *gin.Context, jobID string) (*activities.StoredTranscript, bool) {
	rc, err := h.storage.Get(c.Request.Context(), fmt.Sprintf("jobs/%s/transcript.json", jobID))
	if err != nil {
		return nil, false
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		return nil, false
	}
	var doc activities.StoredTranscript
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, false
	}
	return &doc, true
}
