package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"reel/internal/storage"
)

// RouterDeps bundles optional dependencies for API routes.
type RouterDeps struct {
	Logger        *slog.Logger
	Env           string
	Storage       storage.Strategy
	Jobs          JobService
	MaxFileSizeMB int64
}

// NewRouter builds the Gin engine with middleware and all v1 routes.
func NewRouter(deps RouterDeps) *gin.Engine {
	if deps.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.TestMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	if deps.Logger != nil {
		r.Use(RequestLogger(deps.Logger))
	}
	r.Use(ErrorHandler())

	r.GET("/health", healthHandler)

	jh := &jobHandler{
		storage:       deps.Storage,
		jobs:          deps.Jobs,
		maxFileSizeMB: deps.MaxFileSizeMB,
	}
	th := &transcriptHandler{
		storage: deps.Storage,
	}
	sh := &subtitleHandler{
		storage: deps.Storage,
	}
	sbh := &storyboardHandler{
		storage: deps.Storage,
	}

	v1 := r.Group("/api/v1")
	{
		v1.POST("/jobs/upload", jh.upload)
		v1.GET("/jobs/:id", jh.getStatus)
		v1.DELETE("/jobs/:id", jh.cancel)
		v1.GET("/jobs/:id/transcript", th.get)
		v1.PUT("/jobs/:id/transcript", th.update)
		v1.GET("/jobs/:id/subtitles", sh.get)
		v1.PUT("/jobs/:id/subtitles/style", sh.updateStyle)
		v1.GET("/jobs/:id/storyboard", sbh.get)
	}

	return r
}
