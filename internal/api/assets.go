package api

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"reel/internal/storage"
)

var assetBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 64*1024)
		return &b
	},
}

type assetHandler struct {
	storage storage.Strategy
}

func (h *assetHandler) get(c *gin.Context) {
	if h.storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage not configured"})
		return
	}

	jobID := c.Param("id")
	relPath := strings.TrimPrefix(c.Param("asset"), "/")
	if relPath == "" || strings.Contains(relPath, "..") || strings.Contains(jobID, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset path"})
		return
	}

	key := path.Clean(fmt.Sprintf("jobs/%s/%s", jobID, relPath))
	if !strings.HasPrefix(key, fmt.Sprintf("jobs/%s/", jobID)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset path"})
		return
	}

	rc, err := h.storage.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
		return
	}
	defer rc.Close()

	switch strings.ToLower(path.Ext(key)) {
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".mp4":
		c.Header("Content-Type", "video/mp4")
	case ".wav":
		c.Header("Content-Type", "audio/wav")
	case ".json":
		c.Header("Content-Type", "application/json")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}

	c.Status(http.StatusOK)
	bufPtr := assetBufPool.Get().(*[]byte)
	defer assetBufPool.Put(bufPtr)
	_, _ = io.CopyBuffer(c.Writer, rc, *bufPtr)
}
