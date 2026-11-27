package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"reel/internal/ai/asr"
	"reel/internal/storage"
	"reel/internal/temporal/activities"
)

type transcriptHandler struct {
	storage storage.Strategy
}

func (h *transcriptHandler) get(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	key := fmt.Sprintf("jobs/%s/transcript.json", jobID)

	rc, err := h.storage.Get(c.Request.Context(), key)
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
	c.JSON(http.StatusOK, doc)
}

func (h *transcriptHandler) update(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	key := fmt.Sprintf("jobs/%s/transcript.json", jobID)

	exists, err := h.storage.Exists(c.Request.Context(), key)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("transcript for job %q not found", jobID)})
		return
	}

	var edited asr.TranscribeResponse
	if err := c.ShouldBindJSON(&edited); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid transcript payload: %v", err)})
		return
	}

	asr.RebuildTranscriptFromSegments(&edited)
	quality := asr.ValidateTranscript(&edited)

	doc := activities.StoredTranscript{
		Transcript: &edited,
		Quality:    quality,
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize transcript"})
		return
	}

	if err := h.storage.Put(c.Request.Context(), key, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save transcript"})
		return
	}

	c.JSON(http.StatusOK, doc)
}
