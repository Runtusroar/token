package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

// UserHandler provides HTTP handlers for user-facing endpoints.
type UserHandler struct {
	UserService    *service.UserService
	ApiKeyService  *service.ApiKeyService
	BillingService *service.BillingService
	RequestLogRepo *repository.RequestLogRepo
	BalanceLogRepo *repository.BalanceLogRepo
}

// maskKey returns the first 7 chars + "****" + last 4 chars of the key.
// Keys shorter than 11 characters are returned as-is.
func maskKey(key string) string {
	if len(key) < 11 {
		return key
	}
	return key[:7] + "****" + key[len(key)-4:]
}

// ─── Profile ─────────────────────────────────────────────────────────────────

// GetProfile returns the authenticated user's profile.
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := getUserID(c)
	user, err := h.UserService.GetProfile(userID)
	if err != nil {
		pkg.NotFound(c, "user not found")
		return
	}
	pkg.OK(c, user)
}

// ChangePassword updates the authenticated user's password.
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)
	if err := h.UserService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, nil)
}

// ─── API Keys ─────────────────────────────────────────────────────────────────

// ListApiKeys returns the user's API keys with the key value masked.
func (h *UserHandler) ListApiKeys(c *gin.Context) {
	userID := getUserID(c)
	keys, err := h.ApiKeyService.List(userID)
	if err != nil {
		pkg.InternalError(c, "failed to list api keys")
		return
	}

	type maskedKey struct {
		ID         int64       `json:"id"`
		Name       string      `json:"name"`
		Key        string      `json:"key"`
		Status     string      `json:"status"`
		CreatedAt  interface{} `json:"created_at"`
		LastUsedAt interface{} `json:"last_used_at"`
	}

	result := make([]maskedKey, len(keys))
	for i, k := range keys {
		result[i] = maskedKey{
			ID:         k.ID,
			Name:       k.Name,
			Key:        maskKey(k.Key),
			Status:     k.Status,
			CreatedAt:  k.CreatedAt,
			LastUsedAt: k.LastUsedAt,
		}
	}
	pkg.OK(c, result)
}

// CreateApiKey creates a new API key for the authenticated user.
func (h *UserHandler) CreateApiKey(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)
	key, err := h.ApiKeyService.Create(userID, req.Name)
	if err != nil {
		pkg.InternalError(c, "failed to create api key")
		return
	}

	// Return the full key once — the caller must copy it now.
	pkg.Created(c, key)
}

// DeleteApiKey deletes one of the user's API keys.
func (h *UserHandler) DeleteApiKey(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid id")
		return
	}

	userID := getUserID(c)
	if err := h.ApiKeyService.Delete(id, userID); err != nil {
		pkg.InternalError(c, "failed to delete api key")
		return
	}
	pkg.OK(c, nil)
}

// UpdateApiKey updates the status of one of the user's API keys.
func (h *UserHandler) UpdateApiKey(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)
	if err := h.ApiKeyService.UpdateStatus(id, userID, req.Status); err != nil {
		pkg.InternalError(c, "failed to update api key")
		return
	}
	pkg.OK(c, nil)
}

// ─── Logs ─────────────────────────────────────────────────────────────────────

// ListLogs returns a paginated list of the user's request logs.
func (h *UserHandler) ListLogs(c *gin.Context) {
	userID := getUserID(c)
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	logs, total, err := h.RequestLogRepo.ListByUser(userID, page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list logs")
		return
	}

	pkg.OK(c, gin.H{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListBalanceLogs returns a paginated list of the user's balance change history.
func (h *UserHandler) ListBalanceLogs(c *gin.Context) {
	userID := getUserID(c)
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	logs, total, err := h.BalanceLogRepo.ListByUser(userID, page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list balance logs")
		return
	}

	pkg.OK(c, gin.H{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ─── Billing ──────────────────────────────────────────────────────────────────

// Redeem validates and applies a redemption code to the user's balance.
func (h *UserHandler) Redeem(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)
	if err := h.BillingService.Redeem(userID, req.Code); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}
	pkg.OK(c, nil)
}

// Dashboard returns the user's balance and today's usage statistics.
func (h *UserHandler) Dashboard(c *gin.Context) {
	userID := getUserID(c)
	data, err := h.UserService.Dashboard(userID)
	if err != nil {
		pkg.InternalError(c, "failed to load dashboard")
		return
	}
	pkg.OK(c, data)
}

// ─── Route Registration ───────────────────────────────────────────────────────

// RegisterRoutes mounts all user-facing routes under the given router group.
// The group should already be protected by JWTAuth middleware.
func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/profile", h.GetProfile)
	rg.POST("/password", h.ChangePassword)

	keys := rg.Group("/keys")
	{
		keys.GET("", h.ListApiKeys)
		keys.POST("", h.CreateApiKey)
		keys.DELETE("/:id", h.DeleteApiKey)
		keys.PATCH("/:id", h.UpdateApiKey)
	}

	rg.GET("/logs", h.ListLogs)
	rg.GET("/balance-logs", h.ListBalanceLogs)
	rg.POST("/redeem", h.Redeem)
	rg.GET("/dashboard", h.Dashboard)
}
