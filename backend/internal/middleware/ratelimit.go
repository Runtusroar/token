package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit returns a gin middleware that enforces a sliding-window rate limit
// using Redis INCR + EXPIRE. The key is selected in priority order:
//
//  1. rl:ak:<api_key_id>   — authenticated API key request
//  2. rl:u:<user_id>       — authenticated user (JWT) request
//  3. rl:ip:<client_ip>    — unauthenticated request
//
// If rdb is nil or Redis is unavailable the middleware is a no-op and all
// requests are allowed through. Returns 429 when the limit is exceeded.
func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If Redis is not configured, skip rate limiting.
		if rdb == nil {
			c.Next()
			return
		}

		// Derive the rate-limit key from context identifiers.
		key := rateLimitKey(c)

		ctx := context.Background()

		// Increment the counter atomically.
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis is down — fail open and allow the request.
			c.Next()
			return
		}

		// Set the TTL only on the first request in the window to avoid
		// resetting the window on every request.
		if count == 1 {
			if expErr := rdb.Expire(ctx, key, window).Err(); expErr != nil {
				// Non-fatal; the key will just persist until Redis evicts it.
			}
		}

		// Set standard rate-limit headers.
		remaining := int64(maxRequests) - count
		if remaining < 0 {
			remaining = 0
		}
		ttl, _ := rdb.TTL(ctx, key).Result()
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if ttl > 0 {
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(ttl).Unix()))
		}

		if count > int64(maxRequests) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"type":    "rate_limit_error",
					"message": fmt.Sprintf("rate limit exceeded: %d requests per %s", maxRequests, window),
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// rateLimitKey returns the Redis key to use for the given request, in priority
// order: api_key_id → user_id → client IP.
func rateLimitKey(c *gin.Context) string {
	if akID, exists := c.Get("api_key_id"); exists {
		return fmt.Sprintf("rl:ak:%v", akID)
	}
	if userID, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("rl:u:%v", userID)
	}
	return fmt.Sprintf("rl:ip:%s", c.ClientIP())
}
