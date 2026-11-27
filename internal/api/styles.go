package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"reel/internal/story"
)

func listStylesHandler(c *gin.Context) {
	supported := story.SupportedStyles()
	presets := make([]story.StylePreset, 0, len(supported))
	for _, s := range supported {
		presets = append(presets, story.GetStylePreset(s))
	}
	c.JSON(http.StatusOK, gin.H{
		"styles": presets,
	})
}
