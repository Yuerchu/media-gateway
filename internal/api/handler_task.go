package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/internal/task"
)

// TaskHandler handles async task endpoints.
type TaskHandler struct {
	manager *task.Manager
	worker  *task.Worker
}

// NewTaskHandler creates a task handler.
func NewTaskHandler(manager *task.Manager, worker *task.Worker) *TaskHandler {
	return &TaskHandler{
		manager: manager,
		worker:  worker,
	}
}

// CreateTask handles POST /api/v1/tasks.
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req task.TaskRequest

	// Try to get body from middleware (if HMAC auth consumed it)
	if rawBody, ok := c.Get("rawBody"); ok {
		if body, ok := rawBody.([]byte); ok {
			if err := c.ShouldBindJSON(&req); err != nil {
				// Try parsing from raw body
				_ = body
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
				return
			}
		}
	} else if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Validate task type
	switch req.Type {
	case task.TaskTypeThumbnail, task.TaskTypeMetadata, task.TaskTypeTranscode:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task type, must be: thumbnail, metadata, or transcode"})
		return
	}

	// Validate source
	if req.Source.Bucket == "" || req.Source.Key == "" || req.Source.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source bucket, key, and endpoint are required"})
		return
	}

	t := h.manager.Create(req)

	if !h.worker.Submit(t.ID) {
		h.manager.Delete(t.ID)
		c.Header("Retry-After", "5")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "task queue is full"})
		return
	}

	c.JSON(http.StatusAccepted, task.TaskResponse{
		TaskID:    t.ID,
		Status:    t.Status,
		CreatedAt: t.CreatedAt,
	})
}

// GetTask handles GET /api/v1/tasks/:id.
func (h *TaskHandler) GetTask(c *gin.Context) {
	id := c.Param("id")

	t, err := h.manager.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, task.TaskResponse{
		TaskID:    t.ID,
		Status:    t.Status,
		Result:    t.Result,
		Error:     t.Error,
		CreatedAt: t.CreatedAt,
	})
}

// DeleteTask handles DELETE /api/v1/tasks/:id.
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id := c.Param("id")

	if h.manager.Delete(id) {
		c.Status(http.StatusNoContent)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
	}
}
