package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

// AdminHandler groups all admin-only HTTP handlers.
type AdminHandler struct {
	AdminService   *service.AdminService
	BillingService *service.BillingService
	ChannelRepo    *repository.ChannelRepo
	ModelRepo      *repository.ModelConfigRepo
	RedeemRepo     *repository.RedemptionCodeRepo
	RequestLogRepo *repository.RequestLogRepo
	Config         *config.Config
}

// ── Dashboard ──────────────────────────────────────────────────────────────

// Dashboard godoc: GET /api/v1/admin/dashboard
func (h *AdminHandler) Dashboard(c *gin.Context) {
	stats, err := h.AdminService.Dashboard()
	if err != nil {
		pkg.InternalError(c, "failed to fetch dashboard stats")
		return
	}
	pkg.OK(c, stats)
}

// ── Users ──────────────────────────────────────────────────────────────────

// ListUsers godoc: GET /api/v1/admin/users?search=&page=&page_size=
func (h *AdminHandler) ListUsers(c *gin.Context) {
	search := c.Query("search")
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	users, total, err := h.AdminService.ListUsers(page, pageSize, search)
	if err != nil {
		pkg.InternalError(c, "failed to list users")
		return
	}

	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     users,
	})
}

// UpdateUser godoc: PATCH /api/v1/admin/users/:id
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid user id")
		return
	}

	var body struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, "invalid request body")
		return
	}

	user, err := h.AdminService.UpdateUser(id, body.Role, body.Status)
	if err != nil {
		pkg.InternalError(c, err.Error())
		return
	}
	pkg.OK(c, user)
}

// TopUp godoc: POST /api/v1/admin/users/:id/topup
func (h *AdminHandler) TopUp(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid user id")
		return
	}

	var body struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, "invalid request body")
		return
	}
	if body.Amount <= 0 {
		pkg.BadRequest(c, "amount must be greater than 0")
		return
	}

	amount := decimal.NewFromFloat(body.Amount)
	if err := h.AdminService.UserRepo.AddBalance(id, amount); err != nil {
		pkg.InternalError(c, "failed to top up balance")
		return
	}

	pkg.OK(c, gin.H{"message": "balance updated"})
}

// ── Channels ───────────────────────────────────────────────────────────────

// ListChannels godoc: GET /api/v1/admin/channels
func (h *AdminHandler) ListChannels(c *gin.Context) {
	channels, err := h.ChannelRepo.List()
	if err != nil {
		pkg.InternalError(c, "failed to list channels")
		return
	}
	pkg.OK(c, channels)
}

// CreateChannel godoc: POST /api/v1/admin/channels
func (h *AdminHandler) CreateChannel(c *gin.Context) {
	var body struct {
		Name    string   `json:"name"    binding:"required"`
		Type    string   `json:"type"    binding:"required"`
		APIKey  string   `json:"api_key" binding:"required"`
		BaseURL string   `json:"base_url"`
		Models  []string `json:"models"`
		Priority int     `json:"priority"`
		Weight  int      `json:"weight"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	// Encrypt the upstream API key before storage.
	encryptedKey, err := pkg.Encrypt(body.APIKey, h.Config.EncryptionKey)
	if err != nil {
		pkg.InternalError(c, "failed to encrypt api key")
		return
	}

	// Encode models slice as JSON.
	modelsJSON, err := json.Marshal(body.Models)
	if err != nil {
		pkg.InternalError(c, "failed to encode models")
		return
	}

	weight := body.Weight
	if weight < 1 {
		weight = 1
	}

	channel := &model.Channel{
		Name:     body.Name,
		Type:     body.Type,
		ApiKey:   encryptedKey,
		BaseURL:  body.BaseURL,
		Models:   modelsJSON,
		Status:   "active",
		Priority: body.Priority,
		Weight:   weight,
	}

	if err := h.ChannelRepo.Create(channel); err != nil {
		pkg.InternalError(c, "failed to create channel")
		return
	}
	pkg.Created(c, channel)
}

// UpdateChannel godoc: PATCH /api/v1/admin/channels/:id
func (h *AdminHandler) UpdateChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid channel id")
		return
	}

	channel, err := h.ChannelRepo.FindByID(id)
	if err != nil {
		pkg.NotFound(c, "channel not found")
		return
	}

	var body struct {
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		APIKey   string   `json:"api_key"`
		BaseURL  string   `json:"base_url"`
		Models   []string `json:"models"`
		Priority *int     `json:"priority"`
		Weight   *int     `json:"weight"`
		Status   string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, "invalid request body")
		return
	}

	if body.Name != "" {
		channel.Name = body.Name
	}
	if body.Type != "" {
		channel.Type = body.Type
	}
	if body.APIKey != "" {
		encrypted, err := pkg.Encrypt(body.APIKey, h.Config.EncryptionKey)
		if err != nil {
			pkg.InternalError(c, "failed to encrypt api key")
			return
		}
		channel.ApiKey = encrypted
	}
	if body.BaseURL != "" {
		channel.BaseURL = body.BaseURL
	}
	if body.Models != nil {
		modelsJSON, err := json.Marshal(body.Models)
		if err != nil {
			pkg.InternalError(c, "failed to encode models")
			return
		}
		channel.Models = modelsJSON
	}
	if body.Priority != nil {
		channel.Priority = *body.Priority
	}
	if body.Weight != nil {
		w := *body.Weight
		if w < 1 {
			w = 1
		}
		channel.Weight = w
	}
	if body.Status != "" {
		channel.Status = body.Status
	}

	if err := h.ChannelRepo.Update(channel); err != nil {
		pkg.InternalError(c, "failed to update channel")
		return
	}
	pkg.OK(c, channel)
}

// DeleteChannel godoc: DELETE /api/v1/admin/channels/:id
func (h *AdminHandler) DeleteChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid channel id")
		return
	}

	if err := h.ChannelRepo.Delete(id); err != nil {
		pkg.InternalError(c, "failed to delete channel")
		return
	}
	pkg.OK(c, gin.H{"message": "channel deleted"})
}

// TestChannel godoc: POST /api/v1/admin/channels/:id/test
// Decrypts the stored API key to verify the channel configuration is intact.
func (h *AdminHandler) TestChannel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid channel id")
		return
	}

	channel, err := h.ChannelRepo.FindByID(id)
	if err != nil {
		pkg.NotFound(c, "channel not found")
		return
	}

	// Verify the encrypted key can be decrypted correctly.
	decrypted, err := pkg.Decrypt(channel.ApiKey, h.Config.EncryptionKey)
	if err != nil || decrypted == "" {
		pkg.OK(c, gin.H{"success": false, "message": "api key decryption failed"})
		return
	}

	pkg.OK(c, gin.H{"success": true, "message": "channel configuration is valid"})
}

// ── Models ─────────────────────────────────────────────────────────────────

// ListModels godoc: GET /api/v1/admin/models
func (h *AdminHandler) ListModels(c *gin.Context) {
	models, err := h.ModelRepo.List()
	if err != nil {
		pkg.InternalError(c, "failed to list models")
		return
	}
	pkg.OK(c, models)
}

// CreateModel godoc: POST /api/v1/admin/models
func (h *AdminHandler) CreateModel(c *gin.Context) {
	var body struct {
		ModelName   string  `json:"model_name"   binding:"required"`
		DisplayName string  `json:"display_name"`
		Rate        float64 `json:"rate"`
		InputPrice  float64 `json:"input_price"`
		OutputPrice float64 `json:"output_price"`
		Enabled     bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	rate := decimal.NewFromFloat(body.Rate)
	if rate.LessThanOrEqual(decimal.Zero) {
		rate = decimal.NewFromFloat(1.0)
	}

	cfg := &model.ModelConfig{
		ModelName:   body.ModelName,
		DisplayName: body.DisplayName,
		Rate:        rate,
		InputPrice:  decimal.NewFromFloat(body.InputPrice),
		OutputPrice: decimal.NewFromFloat(body.OutputPrice),
		Enabled:     body.Enabled,
	}

	if err := h.ModelRepo.Create(cfg); err != nil {
		pkg.InternalError(c, "failed to create model")
		return
	}
	pkg.Created(c, cfg)
}

// UpdateModel godoc: PATCH /api/v1/admin/models/:id
func (h *AdminHandler) UpdateModel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid model id")
		return
	}

	// Fetch all models and find the matching one (no FindByID in repo).
	all, err := h.ModelRepo.List()
	if err != nil {
		pkg.InternalError(c, "failed to fetch models")
		return
	}

	var cfg *model.ModelConfig
	for i := range all {
		if all[i].ID == id {
			cfg = &all[i]
			break
		}
	}
	if cfg == nil {
		pkg.NotFound(c, "model not found")
		return
	}

	var body struct {
		ModelName   string   `json:"model_name"`
		DisplayName string   `json:"display_name"`
		Rate        *float64 `json:"rate"`
		InputPrice  *float64 `json:"input_price"`
		OutputPrice *float64 `json:"output_price"`
		Enabled     *bool    `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, "invalid request body")
		return
	}

	if body.ModelName != "" {
		cfg.ModelName = body.ModelName
	}
	if body.DisplayName != "" {
		cfg.DisplayName = body.DisplayName
	}
	if body.Rate != nil {
		r := decimal.NewFromFloat(*body.Rate)
		if r.LessThanOrEqual(decimal.Zero) {
			r = decimal.NewFromFloat(1.0)
		}
		cfg.Rate = r
	}
	if body.InputPrice != nil {
		cfg.InputPrice = decimal.NewFromFloat(*body.InputPrice)
	}
	if body.OutputPrice != nil {
		cfg.OutputPrice = decimal.NewFromFloat(*body.OutputPrice)
	}
	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	}

	if err := h.ModelRepo.Update(cfg); err != nil {
		pkg.InternalError(c, "failed to update model")
		return
	}
	pkg.OK(c, cfg)
}

// ── Redeem Codes ───────────────────────────────────────────────────────────

// ListRedeemCodes godoc: GET /api/v1/admin/redeem-codes?page=&page_size=
func (h *AdminHandler) ListRedeemCodes(c *gin.Context) {
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	codes, total, err := h.RedeemRepo.List(page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list redeem codes")
		return
	}

	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     codes,
	})
}

// CreateRedeemCodes godoc: POST /api/v1/admin/redeem-codes
// Body: { amount: float, count: int (1-100), expires_at: "2006-01-02" (optional) }
func (h *AdminHandler) CreateRedeemCodes(c *gin.Context) {
	var body struct {
		Amount    float64 `json:"amount"     binding:"required"`
		Count     int     `json:"count"      binding:"required,min=1,max=100"`
		ExpiresAt string  `json:"expires_at"` // optional date string YYYY-MM-DD
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	adminID := getUserID(c)
	amount := decimal.NewFromFloat(body.Amount)

	var expiresAt *time.Time
	if body.ExpiresAt != "" {
		t, err := time.Parse("2006-01-02", body.ExpiresAt)
		if err != nil {
			pkg.BadRequest(c, "invalid expires_at format, use YYYY-MM-DD")
			return
		}
		expiresAt = &t
	}

	created := make([]model.RedemptionCode, 0, body.Count)
	for i := 0; i < body.Count; i++ {
		code := &model.RedemptionCode{
			Code:      pkg.GenerateRedeemCode(),
			Amount:    amount,
			Status:    "unused",
			CreatedBy: adminID,
			ExpiresAt: expiresAt,
		}
		if err := h.RedeemRepo.Create(code); err != nil {
			pkg.InternalError(c, "failed to create redeem code")
			return
		}
		created = append(created, *code)
	}

	pkg.Created(c, created)
}

// UpdateRedeemCode godoc: PATCH /api/v1/admin/redeem-codes/:id
func (h *AdminHandler) UpdateRedeemCode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid redeem code id")
		return
	}

	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	// List all codes to find the one matching the id (no FindByID in repo).
	// Use page=1, size=1 hack won't work; we scan pages or use a direct query.
	// The RedemptionCodeRepo only has List+FindByCode+Update. We must scan.
	// Since this is an admin operation (infrequent), a full scan is acceptable,
	// but we'll pull with a large page size to avoid N calls.
	codes, _, err := h.RedeemRepo.List(1, 1000)
	if err != nil {
		pkg.InternalError(c, "failed to fetch redeem codes")
		return
	}

	var target *model.RedemptionCode
	for i := range codes {
		if codes[i].ID == id {
			target = &codes[i]
			break
		}
	}
	if target == nil {
		pkg.NotFound(c, "redeem code not found")
		return
	}

	target.Status = body.Status
	if err := h.RedeemRepo.Update(target); err != nil {
		pkg.InternalError(c, "failed to update redeem code")
		return
	}
	pkg.OK(c, target)
}

// ── Request Logs ───────────────────────────────────────────────────────────

// ListLogs godoc: GET /api/v1/admin/logs?page=&page_size=&user_id=&model=
func (h *AdminHandler) ListLogs(c *gin.Context) {
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	var userID int64
	if uid := c.Query("user_id"); uid != "" {
		userID, _ = strconv.ParseInt(uid, 10, 64)
	}
	modelFilter := c.Query("model")

	logs, total, err := h.RequestLogRepo.ListAll(page, pageSize, userID, modelFilter)
	if err != nil {
		pkg.InternalError(c, "failed to list logs")
		return
	}

	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     logs,
	})
}

// ── Settings ───────────────────────────────────────────────────────────────

// GetSettings godoc: GET /api/v1/admin/settings
func (h *AdminHandler) GetSettings(c *gin.Context) {
	settings, err := h.AdminService.GetSettings()
	if err != nil {
		pkg.InternalError(c, "failed to fetch settings")
		return
	}
	pkg.OK(c, settings)
}

// UpdateSettings godoc: PUT /api/v1/admin/settings
// Body: map[string]string of key→value pairs to upsert.
func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, "invalid request body")
		return
	}

	for key, value := range body {
		if err := h.AdminService.UpdateSetting(key, value); err != nil {
			pkg.InternalError(c, "failed to update setting: "+key)
			return
		}
	}

	pkg.OK(c, gin.H{"message": "settings updated", "count": len(body)})
}

