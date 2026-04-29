package middleware

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/pkg/signing"
)

const maxTimestampSkew = 5 * time.Minute

// HMACAuth returns a Gin middleware that validates HMAC-SHA256 request signatures.
// Requests must include X-MG-Timestamp and X-MG-Signature headers.
func HMACAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.Next()
			return
		}

		timestamp := c.GetHeader("X-MG-Timestamp")
		sig := c.GetHeader("X-MG-Signature")

		if timestamp == "" || sig == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing X-MG-Timestamp or X-MG-Signature header",
			})
			return
		}

		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid X-MG-Timestamp",
			})
			return
		}

		skew := time.Duration(math.Abs(float64(time.Now().Unix()-ts))) * time.Second
		if skew > maxTimestampSkew {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "timestamp expired",
			})
			return
		}

		// Read body for signature verification
		body, err := c.GetRawData()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "failed to read request body",
			})
			return
		}

		bodySHA := signing.SHA256Hex(body)
		expected := signing.SignRequest(c.Request.Method, c.Request.URL.Path, timestamp, bodySHA, secret)

		if sig != expected {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid signature",
			})
			return
		}

		// Put body back for downstream handlers
		c.Request.Body = http.NoBody
		c.Set("rawBody", body)
		c.Next()
	}
}
