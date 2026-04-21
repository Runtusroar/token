package handler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

// AdminHandler groups all admin-only HTTP handlers.
type AdminHandler struct {
	DB             *gorm.DB
	AdminService   *service.AdminService
	BillingService *service.BillingService
	ChannelService *service.ChannelService
	ChannelRepo    *repository.ChannelRepo
	ModelRepo      *repository.ModelConfigRepo
	RedeemRepo     *repository.RedemptionCodeRepo
	RequestLogRepo *repository.RequestLogRepo
	BalanceLogRepo *repository.BalanceLogRepo
	Config         *config.Config
}

// audit logs an admin action asynchronously.
func (h *AdminHandler) audit(c *gin.Context, action, target string, targetID int64, detail string) {
	entry := &model.AuditLog{
		AdminID:  getUserID(c),
		Action:   action,
		Target:   target,
		TargetID: targetID,
		Detail:   detail,
		IP:       c.ClientIP(),
	}
	go h.DB.Create(entry) //nolint:errcheck
}

// ── Dashboard ──────────────────────────────────────────────────────────────

// DailyFinance holds one day's financial data.
type DailyFinance struct {
	Date          string `json:"date"`
	RedeemIncome  string `json:"redeem_income"`
	TopupIncome   string `json:"topup_income"`
	Consumption   string `json:"consumption"`
	UpstreamCost  string `json:"upstream_cost"`
	Profit        string `json:"profit"`
	Requests      int64  `json:"requests"`
}

// DailyStats godoc: GET /api/admin/daily-stats?days=30
func (h *AdminHandler) DailyStats(c *gin.Context) {
	days := 30
	if d, err := strconv.Atoi(c.DefaultQuery("days", "30")); err == nil && d > 0 && d <= 90 {
		days = d
	}

	// Income per day from balance_logs.
	type incomeRow struct {
		Date   string
		Redeem decimal.Decimal
		Topup  decimal.Decimal
	}
	var incomeRows []incomeRow
	h.DB.Raw(`
		SELECT DATE(created_at) AS date,
		       COALESCE(SUM(CASE WHEN type='redeem' THEN amount ELSE 0 END), 0) AS redeem,
		       COALESCE(SUM(CASE WHEN type='topup'  THEN amount ELSE 0 END), 0) AS topup
		FROM balance_logs
		WHERE created_at >= CURRENT_DATE - INTERVAL '1 day' * ?
		GROUP BY DATE(created_at)
	`, days).Scan(&incomeRows)

	// Expense per day from request_logs.
	type expenseRow struct {
		Date        string
		Consumption decimal.Decimal
		Upstream    decimal.Decimal
		Requests    int64
	}
	var expenseRows []expenseRow
	h.DB.Raw(`
		SELECT DATE(created_at) AS date,
		       COALESCE(SUM(cost), 0) AS consumption,
		       COALESCE(SUM(upstream_cost), 0) AS upstream,
		       COUNT(*) AS requests
		FROM request_logs
		WHERE status='success' AND created_at >= CURRENT_DATE - INTERVAL '1 day' * ?
		GROUP BY DATE(created_at)
	`, days).Scan(&expenseRows)

	// Merge into a map keyed by date string.
	type merged struct {
		redeem, topup, consumption, upstream decimal.Decimal
		requests                             int64
	}
	m := make(map[string]*merged)
	for _, r := range incomeRows {
		m[r.Date] = &merged{redeem: r.Redeem, topup: r.Topup}
	}
	for _, r := range expenseRows {
		if _, ok := m[r.Date]; !ok {
			m[r.Date] = &merged{}
		}
		m[r.Date].consumption = r.Consumption
		m[r.Date].upstream = r.Upstream
		m[r.Date].requests = r.Requests
	}

	// Sort by date descending.
	dates := make([]string, 0, len(m))
	for d := range m {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	result := make([]DailyFinance, 0, len(dates))
	for _, d := range dates {
		v := m[d]
		result = append(result, DailyFinance{
			Date:         d,
			RedeemIncome: v.redeem.String(),
			TopupIncome:  v.topup.String(),
			Consumption:  v.consumption.String(),
			UpstreamCost: v.upstream.String(),
			Profit:       v.consumption.Sub(v.upstream).String(),
			Requests:     v.requests,
		})
	}

	pkg.OK(c, result)
}

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

// userWithConsumption extends the User model with aggregated usage stats.
type userWithConsumption struct {
	model.User
	RequestCount int64           `json:"request_count"`
	TotalTokens  int64           `json:"total_tokens"`
	TotalCost    decimal.Decimal `json:"total_cost"`
}

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

	// Collect user IDs and fetch consumption stats.
	ids := make([]int64, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}

	consumption, _ := h.RequestLogRepo.ConsumptionByUsers(ids)

	items := make([]userWithConsumption, len(users))
	for i, u := range users {
		items[i] = userWithConsumption{User: u}
		if cs, ok := consumption[u.ID]; ok {
			items[i].RequestCount = cs.RequestCount
			items[i].TotalTokens = cs.TotalTokens
			items[i].TotalCost = cs.TotalCost
		}
	}

	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     items,
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
		Role           string   `json:"role"`
		Status         string   `json:"status"`
		RateMultiplier *float64 `json:"rate_multiplier"`
		Note           *string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		pkg.BadRequest(c, "invalid request body")
		return
	}

	var rate *decimal.Decimal
	if body.RateMultiplier != nil {
		if *body.RateMultiplier < 0 {
			pkg.BadRequest(c, "rate_multiplier must be >= 0")
			return
		}
		d := decimal.NewFromFloat(*body.RateMultiplier)
		rate = &d
	}

	user, err := h.AdminService.UpdateUser(id, body.Role, body.Status, rate, body.Note)
	if err != nil {
		pkg.InternalError(c, err.Error())
		return
	}
	rateStr := "-"
	if rate != nil {
		rateStr = rate.String()
	}
	noteChanged := ""
	if body.Note != nil {
		noteChanged = " note_updated"
	}
	h.audit(c, "update_user", "user", id, fmt.Sprintf("role=%s status=%s rate=%s%s", body.Role, body.Status, rateStr, noteChanged))
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
	adminID := getUserID(c)
	if err := h.BillingService.AdminTopUp(id, amount, adminID); err != nil {
		pkg.InternalError(c, "failed to top up balance")
		return
	}
	h.audit(c, "topup", "user", id, fmt.Sprintf("amount=%s", amount.String()))
	pkg.OK(c, gin.H{"message": "balance updated"})
}

// UserBalanceLogs godoc: GET /api/admin/users/:id/balance-logs?page=&page_size=
func (h *AdminHandler) UserBalanceLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid user id")
		return
	}
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	logs, total, err := h.BalanceLogRepo.ListByUser(id, page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list balance logs")
		return
	}
	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     logs,
	})
}

// UserRequestLogs godoc: GET /api/admin/users/:id/request-logs?page=&page_size=
func (h *AdminHandler) UserRequestLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid user id")
		return
	}
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	logs, total, err := h.RequestLogRepo.ListByUser(id, page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list request logs")
		return
	}
	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     logs,
	})
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
	h.ChannelService.InvalidateCache()
	h.audit(c, "create_channel", "channel", channel.ID, channel.Name)
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
	h.ChannelService.InvalidateCache()
	h.audit(c, "update_channel", "channel", id, channel.Name)
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
	h.ChannelService.InvalidateCache()
	h.audit(c, "delete_channel", "channel", id, "")
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
		Provider    string  `json:"provider"`
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

	provider := body.Provider
	if provider == "" {
		provider = "claude"
	}

	cfg := &model.ModelConfig{
		ModelName:   body.ModelName,
		Provider:    provider,
		DisplayName: body.DisplayName,
		Rate:        rate,
		InputPrice:  decimal.NewFromFloat(body.InputPrice),
		OutputPrice: decimal.NewFromFloat(body.OutputPrice),
		Enabled:     true,
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

	cfg, err := h.ModelRepo.FindByID(id)
	if err != nil {
		pkg.NotFound(c, "model not found")
		return
	}

	var body struct {
		ModelName   string   `json:"model_name"`
		Provider    string   `json:"provider"`
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
	if body.Provider != "" {
		cfg.Provider = body.Provider
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

// DeleteModel godoc: DELETE /api/admin/models/:id
func (h *AdminHandler) DeleteModel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		pkg.BadRequest(c, "invalid model id")
		return
	}

	if err := h.ModelRepo.Delete(id); err != nil {
		pkg.InternalError(c, "failed to delete model")
		return
	}
	h.audit(c, "delete_model", "model", id, "")
	pkg.OK(c, gin.H{"message": "model deleted"})
}

// ── Redeem Codes ───────────────────────────────────────────────────────────

// redeemCodeWithEmail extends RedemptionCode with the user's email.
type redeemCodeWithEmail struct {
	model.RedemptionCode
	UsedByEmail string `json:"used_by_email"`
}

// ListRedeemCodes godoc: GET /api/v1/admin/redeem-codes?page=&page_size=
func (h *AdminHandler) ListRedeemCodes(c *gin.Context) {
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	codes, total, err := h.RedeemRepo.List(page, pageSize)
	if err != nil {
		pkg.InternalError(c, "failed to list redeem codes")
		return
	}

	// Collect used_by IDs and batch-query emails.
	var userIDs []int64
	for _, code := range codes {
		if code.UsedBy != nil {
			userIDs = append(userIDs, *code.UsedBy)
		}
	}

	emailMap := make(map[int64]string)
	if len(userIDs) > 0 {
		var users []model.User
		h.AdminService.UserRepo.DB.Select("id, email").Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			emailMap[u.ID] = u.Email
		}
	}

	items := make([]redeemCodeWithEmail, len(codes))
	for i, code := range codes {
		items[i] = redeemCodeWithEmail{RedemptionCode: code}
		if code.UsedBy != nil {
			items[i].UsedByEmail = emailMap[*code.UsedBy]
		}
	}

	pkg.OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     items,
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

	target, err := h.RedeemRepo.FindByID(id)
	if err != nil {
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

	h.audit(c, "update_settings", "settings", 0, fmt.Sprintf("keys=%d", len(body)))
	pkg.OK(c, gin.H{"message": "settings updated", "count": len(body)})
}

