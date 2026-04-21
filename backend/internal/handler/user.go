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
	ModelRepo      *repository.ModelConfigRepo
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

// ListApiKeys returns the user's API keys with the key value masked and
// per-key consumption stats (request count, tokens, cost).
func (h *UserHandler) ListApiKeys(c *gin.Context) {
	userID := getUserID(c)
	keys, err := h.ApiKeyService.List(userID)
	if err != nil {
		pkg.InternalError(c, "failed to list api keys")
		return
	}

	// Collect key IDs and batch-query consumption.
	keyIDs := make([]int64, len(keys))
	for i, k := range keys {
		keyIDs[i] = k.ID
	}
	consumption, _ := h.RequestLogRepo.ConsumptionByApiKeys(keyIDs)

	type maskedKey struct {
		ID           int64       `json:"id"`
		Name         string      `json:"name"`
		Key          string      `json:"key"`
		Status       string      `json:"status"`
		CreatedAt    interface{} `json:"created_at"`
		LastUsedAt   interface{} `json:"last_used_at"`
		RequestCount int64       `json:"request_count"`
		TotalTokens  int64       `json:"total_tokens"`
		TotalCost    interface{} `json:"total_cost"`
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
		if cs, ok := consumption[k.ID]; ok {
			result[i].RequestCount = cs.RequestCount
			result[i].TotalTokens = cs.TotalTokens
			result[i].TotalCost = cs.TotalCost
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
	// ApiKey.Key has json:"-", so we use a one-off struct to include it.
	pkg.Created(c, gin.H{
		"id":         key.ID,
		"name":       key.Name,
		"key":        key.Key,
		"status":     key.Status,
		"created_at": key.CreatedAt,
	})
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
		"items":     logs,
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
		"items":     logs,
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

// DailyStats returns per-day consumption for the authenticated user.
func (h *UserHandler) DailyStats(c *gin.Context) {
	userID := getUserID(c)
	days := 7
	if d, err := strconv.Atoi(c.DefaultQuery("days", "7")); err == nil && d > 0 && d <= 90 {
		days = d
	}

	stats, err := h.RequestLogRepo.DailyStatsByUser(userID, days)
	if err != nil {
		pkg.InternalError(c, "failed to load daily stats")
		return
	}
	pkg.OK(c, stats)
}

// ListModels returns all enabled models visible to users, with the
// displayed rate already multiplied by the user's personal rate multiplier
// so the value shown matches what they'll actually be billed.
func (h *UserHandler) ListModels(c *gin.Context) {
	userID := getUserID(c)
	userRate, err := h.BillingService.UserRateMultiplier(userID)
	if err != nil {
		pkg.InternalError(c, "failed to load user rate")
		return
	}

	models, err := h.ModelRepo.ListEnabled()
	if err != nil {
		pkg.InternalError(c, "failed to list models")
		return
	}

	for i := range models {
		models[i].Rate = models[i].Rate.Mul(userRate)
	}
	pkg.OK(c, models)
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

