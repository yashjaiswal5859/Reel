package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"reel/internal/storage"
)

type downloadHandler struct {
	storage storage.Strategy
}

func (h *downloadHandler) download(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	if strings.Contains(jobID, "..") || strings.Contains(jobID, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	key := fmt.Sprintf("jobs/%s/final.mp4", jobID)
	exists, err := h.storage.Exists(c.Request.Context(), key)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("final video for job %q not ready", jobID)})
		return
	}

	if c.Query("mode") == "url" {
		u, err := h.storage.URL(c.Request.Context(), key, 1*time.Hour)
		if err == nil && strings.HasPrefix(u, "http") {
			c.JSON(http.StatusOK, gin.H{
				"job_id":       jobID,
				"download_url": u,
				"expires_in":   3600,
			})
			return
		}
	}

	rc, err := h.storage.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "final video not found"})
		return
	}
	defer rc.Close()

	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="reel-%s.mp4"`, jobID))
	c.Status(http.StatusOK)

	bufPtr := assetBufPool.Get().(*[]byte)
	defer assetBufPool.Put(bufPtr)
	_, _ = io.CopyBuffer(c.Writer, rc, *bufPtr)
}
