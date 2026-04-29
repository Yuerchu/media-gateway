package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ConcurrencyLimiter returns a Gin middleware that limits concurrent requests.
// Excess requests are immediately rejected with HTTP 429.
func ConcurrencyLimiter(maxConcurrent int) gin.HandlerFunc {
	sem := make(chan struct{}, maxConcurrent)

	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			c.Next()
		default:
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("server busy, max %d concurrent requests", maxConcurrent),
			})
		}
	}
}
