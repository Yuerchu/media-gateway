package api

import (
	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/internal/config"
	"github.com/yuerchu/media-gateway/internal/middleware"
	"github.com/yuerchu/media-gateway/internal/task"
)

// SetupRouter creates and configures the Gin engine.
//
// It returns an error because the capability contract is rendered here, at
// startup, so that a contract which cannot be built stops the process instead
// of surfacing later as a failing endpoint.
func SetupRouter(cfg *config.Config, manager *task.Manager, worker *task.Worker) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SlogLogger())

	if cfg.Server.MaxConcurrentRequests > 0 {
		r.Use(middleware.ConcurrencyLimiter(cfg.Server.MaxConcurrentRequests))
	}

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1
	v1 := r.Group("/api/v1")

	// Apply HMAC auth if secret is configured
	if cfg.Auth.Secret != "" {
		v1.Use(middleware.HMACAuth(cfg.Auth.Secret))
	}

	// Sync endpoints
	syncH := NewSyncHandler()
	v1.POST("/detect-mime", syncH.DetectMIME)
	v1.POST("/image-meta", syncH.ImageMeta)

	// Task endpoints
	taskH := NewTaskHandler(manager, worker)
	v1.POST("/tasks", taskH.CreateTask)
	v1.GET("/tasks/:id", taskH.GetTask)
	v1.DELETE("/tasks/:id", taskH.DeleteTask)

	// Capability contract. Registered inside the authenticated group on
	// purpose: it describes the task endpoints, its only readers are the
	// callers of those endpoints, and every one of them already holds the
	// secret. Serving deployment limits to anyone who can reach the port buys
	// nothing in exchange.
	capabilityH, err := NewCapabilityHandler(cfg)
	if err != nil {
		return nil, err
	}
	v1.GET("/capabilities", capabilityH.Get)

	return r, nil
}
