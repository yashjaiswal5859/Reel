package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"reel/internal/storage"
	"reel/internal/subtitles"
	"reel/internal/temporal/activities"
)

type subtitleHandler struct {
	storage storage.Strategy
}

func (h *subtitleHandler) get(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	key := fmt.Sprintf("jobs/%s/subtitles.json", jobID)

	rc, err := h.storage.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("subtitles for job %q not found", jobID)})
		return
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read subtitles"})
		return
	}

	var track subtitles.Track
	if err := json.Unmarshal(raw, &track); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt subtitles data"})
		return
	}
	c.JSON(http.StatusOK, track)
}

func (h *subtitleHandler) updateStyle(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	transcriptKey := fmt.Sprintf("jobs/%s/transcript.json", jobID)

	rc, err := h.storage.Get(c.Request.Context(), transcriptKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("transcript for job %q not found", jobID)})
		return
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read transcript"})
		return
	}

	var doc activities.StoredTranscript
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt transcript data"})
		return
	}

	style := subtitles.DefaultStyle()
	if err := c.ShouldBindJSON(&style); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid style payload: %v", err)})
		return
	}

	track, err := subtitles.GenerateTrack(doc.Transcript, style)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jsonBytes, err := json.Marshal(track)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize subtitles"})
		return
	}
	subKey := fmt.Sprintf("jobs/%s/subtitles.json", jobID)
	if err := h.storage.Put(c.Request.Context(), subKey, bytes.NewReader(jsonBytes), int64(len(jsonBytes)), "application/json"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save subtitles"})
		return
	}

	var assBuf bytes.Buffer
	if err := subtitles.RenderASS(&assBuf, track, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render ass"})
		return
	}
	assKey := fmt.Sprintf("jobs/%s/subtitles.ass", jobID)
	if err := h.storage.Put(c.Request.Context(), assKey, bytes.NewReader(assBuf.Bytes()), int64(assBuf.Len()), "text/plain"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save ass file"})
		return
	}

	c.JSON(http.StatusOK, track)
}
