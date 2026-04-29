package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// SlogLogger returns a Gin middleware that logs requests using slog.
func SlogLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		slog.Info("Request completed",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration", time.Since(start).Round(time.Millisecond),
			"size", c.Writer.Size(),
			"client_ip", c.ClientIP(),
		)
	}
}
