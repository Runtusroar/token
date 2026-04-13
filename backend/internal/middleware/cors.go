package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a gin middleware that sets Cross-Origin Resource Sharing headers.
// allowedOrigins is a comma-separated list of allowed origins; pass "*" to
// allow all origins. OPTIONS preflight requests are answered with 204 No Content.
func CORS(allowedOrigins string) gin.HandlerFunc {
	// Parse and normalize the allowed-origins list once at startup.
	rawOrigins := strings.Split(allowedOrigins, ",")
	origins := make([]string, 0, len(rawOrigins))
	allowAll := false
	for _, o := range rawOrigins {
		trimmed := strings.TrimSpace(o)
		if trimmed == "*" {
			allowAll = true
			break
		}
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	isAllowed := func(origin string) bool {
		if allowAll {
			return true
		}
		for _, o := range origins {
			if strings.EqualFold(o, origin) {
				return true
			}
		}
		return false
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" && isAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		// Respond to preflight requests immediately.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
