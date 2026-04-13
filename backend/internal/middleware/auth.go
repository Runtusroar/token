package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
)

// JWTAuth validates the "Authorization: Bearer <token>" header, verifies the
// JWT with the provided secret, and stores user_id, email, and role in the
// gin.Context. Returns 401 on any failure.
func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			pkg.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			pkg.Unauthorized(c, "invalid authorization header format")
			c.Abort()
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		claims, err := pkg.VerifyJWT(tokenStr, jwtSecret)
		if err != nil {
			pkg.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// AdminOnly checks that the "role" value stored in the context is "admin".
// Must be used after JWTAuth. Returns 403 if the role is not admin.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			pkg.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// apiKeyError sends a Claude API–compatible error response.
func apiKeyError(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"type":    "authentication_error",
			"message": message,
		},
	})
	c.Abort()
}

// APIKeyAuth authenticates requests via a relay API key. The key may be
// supplied either in the "x-api-key" header or as "Authorization: Bearer sk-…".
// On success it sets api_key_id, user_id, and balance in the context and
// asynchronously updates last_used_at on the key row.
func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer the x-api-key header; fall back to Authorization: Bearer.
		rawKey := c.GetHeader("x-api-key")
		if rawKey == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					rawKey = strings.TrimSpace(parts[1])
				}
			}
		}

		if rawKey == "" {
			apiKeyError(c, "missing api key")
			return
		}

		// Only accept keys that look like relay keys (sk- prefix).
		if !strings.HasPrefix(rawKey, "sk-") {
			apiKeyError(c, "invalid api key")
			return
		}

		// Join api_keys → users; both must be active.
		type result struct {
			APIKeyID int64
			UserID   int64
			Balance  string
		}

		var row struct {
			model.ApiKey
			UserBalance string
			UserStatus  string
		}

		err := db.Table("api_keys ak").
			Select("ak.id, ak.user_id, ak.status, u.balance as user_balance, u.status as user_status").
			Joins("JOIN users u ON u.id = ak.user_id").
			Where("ak.key = ? AND ak.status = ? AND u.status = ?", rawKey, "active", "active").
			First(&row).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				apiKeyError(c, "invalid or inactive api key")
			} else {
				apiKeyError(c, "internal server error")
			}
			return
		}

		// Store values in context for downstream handlers.
		c.Set("api_key_id", row.ApiKey.ID)
		c.Set("user_id", row.ApiKey.UserID)
		c.Set("balance", row.UserBalance)

		// Update last_used_at in the background to avoid blocking the request.
		keyID := row.ApiKey.ID
		go func() {
			now := time.Now()
			db.Model(&model.ApiKey{}).Where("id = ?", keyID).Update("last_used_at", now)
		}()

		c.Next()
	}
}
