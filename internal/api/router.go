package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"reel/internal/ai/imagegen"
	"reel/internal/storage"
)

// RouterDeps bundles optional dependencies for API routes.
type RouterDeps struct {
	Logger           *slog.Logger
	Env              string
	Storage          storage.Strategy
	Jobs             JobService
	ImageGen         imagegen.Strategy
	MaxFileSizeMB    int64
	MaxRegenAttempts int
	DefaultASR       string
	DefaultLLM       string
	DefaultImageGen  string
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
	r.Use(CORS())
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
		storage:          deps.Storage,
		imageGen:         deps.ImageGen,
		maxRegenAttempts: deps.MaxRegenAttempts,
	}
	ah := &assetHandler{
		storage: deps.Storage,
	}
	dh := &downloadHandler{
		storage: deps.Storage,
	}
	ch := &costHandler{
		storage:         deps.Storage,
		jobs:            deps.Jobs,
		defaultASR:      deps.DefaultASR,
		defaultLLM:      deps.DefaultLLM,
		defaultImageGen: deps.DefaultImageGen,
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/styles", listStylesHandler)
		v1.POST("/estimate", ch.estimate)
		v1.POST("/jobs/upload", jh.upload)
		v1.GET("/jobs/:id", jh.getStatus)
		v1.GET("/jobs/:id/stages", jh.getStages)
		v1.GET("/jobs/:id/cost", ch.getJobCost)
		v1.DELETE("/jobs/:id", jh.cancel)
		v1.GET("/jobs/:id/transcript", th.get)
		v1.PUT("/jobs/:id/transcript", th.update)
		v1.GET("/jobs/:id/subtitles", sh.get)
		v1.PUT("/jobs/:id/subtitles/style", sh.updateStyle)
		v1.GET("/jobs/:id/storyboard", sbh.get)
		v1.PUT("/jobs/:id/storyboard/style", sbh.updateStyle)
		v1.GET("/jobs/:id/storyboard/scenes", sbh.listScenes)
		v1.GET("/jobs/:id/storyboard/scenes/:sceneId", sbh.getScene)
		v1.PATCH("/jobs/:id/storyboard/scenes/:sceneId", sbh.patchScene)
		v1.POST("/jobs/:id/storyboard/scenes/:sceneId/regenerate", sbh.regenerateScene)
		v1.GET("/jobs/:id/assets/*asset", ah.get)
		v1.GET("/jobs/:id/download", dh.download)
	}

	return r
}
