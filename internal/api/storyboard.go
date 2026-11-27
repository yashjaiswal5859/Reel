package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"reel/internal/storage"
	"reel/internal/temporal/activities"
)

type storyboardHandler struct {
	storage storage.Strategy
}

func (h *storyboardHandler) get(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	key := fmt.Sprintf("jobs/%s/storyboard.json", jobID)

	rc, err := h.storage.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("storyboard for job %q not found", jobID)})
		return
	}
	defer rc.Close()

	raw, err := io.ReadAll(io.LimitReader(rc, 10*1024*1024))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read storyboard"})
		return
	}

	var doc activities.StoredStoryboard
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt storyboard data"})
		return
	}
	c.JSON(http.StatusOK, doc)
}
