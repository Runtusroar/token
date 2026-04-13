package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// getUserID extracts the authenticated user's ID from the gin context.
// The value is set by middleware.JWTAuth as int64.
func getUserID(c *gin.Context) int64 {
	v, _ := c.Get("user_id")
	id, _ := v.(int64)
	return id
}

// getPage returns the "page" query parameter as an int, defaulting to 1.
// It always returns at least 1.
func getPage(c *gin.Context) int {
	p, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || p < 1 {
		return 1
	}
	return p
}

// getPageSize returns the "page_size" query parameter as an int, defaulting
// to defaultSize. The returned value is clamped to [1, 100].
func getPageSize(c *gin.Context, defaultSize int) int {
	s, err := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))
	if err != nil || s < 1 {
		return defaultSize
	}
	if s > 100 {
		return 100
	}
	return s
}
