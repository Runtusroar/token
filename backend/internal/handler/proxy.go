package handler

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ai-relay/internal/model"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

// modelsCache caches the /v1/models response to avoid DB queries on every call.
var (
	modelsCacheMu   sync.RWMutex
	modelsCacheData []model.ModelConfig
	modelsCacheTime time.Time
	modelsCacheTTL  = 30 * time.Second
)

// checkBalance verifies the user's balance is positive.
// Returns "" when OK; otherwise a human-readable reason the caller should
// surface as an insufficient_balance / 402 error (and log as preflight).
func checkBalance(c *gin.Context) string {
	balStr, _ := c.Get("balance")
	s, ok := balStr.(string)
	if !ok {
		return ""
	}
	bal, err := decimal.NewFromString(s)
	if err != nil {
		return ""
	}
	if bal.LessThanOrEqual(decimal.Zero) {
		return "your balance is zero or negative, please top up"
	}
	return ""
}

// ProxyHandler exposes the AI proxy endpoints.
type ProxyHandler struct {
	ProxyService *service.ProxyService
	ModelRepo    *repository.ModelConfigRepo
}

// proxyError writes a JSON error body in the Anthropic / OpenAI style and
// aborts the gin chain.
func proxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// preflightErr writes a proxy error response AND persists a matching
// request_logs row tagged with stage, so failures that never reach the
// upstream are still visible to operators. reqType is "native" / "openai_compat"
// and modelName may be empty when the request died before parsing.
func (h *ProxyHandler) preflightErr(c *gin.Context, status int, errType, reqType, modelName, stage, message string) {
	proxyError(c, status, errType, message)
	if h.ProxyService == nil {
		return
	}
	h.ProxyService.LogPreflightError(&service.ProxyRequest{
		UserID:   getUserID(c),
		ApiKeyID: getApiKeyID(c),
		Model:    modelName,
		Type:     reqType,
		IP:       c.ClientIP(),
	}, stage, message)
}

// NativeMessages handles POST /v1/messages (native Claude protocol).
// Forwards the raw body to ProxyService, which handles conversion if the
// selected channel speaks OpenAI.
func (h *ProxyHandler) NativeMessages(c *gin.Context) {
	if msg := checkBalance(c); msg != "" {
		h.preflightErr(c, http.StatusPaymentRequired, "insufficient_balance",
			"native", "", "balance_insufficient", msg)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"native", "", "body_read", "cannot read request body: "+err.Error())
		return
	}
	model := service.ExtractModel(body)
	if model == "" {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"native", "", "missing_model", "model field is required")
		return
	}

	pr := &service.ProxyRequest{
		UserID:        getUserID(c),
		ApiKeyID:      getApiKeyID(c),
		Model:         model,
		Body:          body,
		Stream:        service.ExtractStream(body),
		Type:          "native",
		InboundProto:  "claude",
		IP:            c.ClientIP(),
		ClientHeaders: c.Request.Header,
	}
	h.ProxyService.HandleProxy(c.Request.Context(), c.Writer, pr)
}

// ChatCompletions handles POST /v1/chat/completions (OpenAI-compatible).
// Forwards the raw body. If the selected channel speaks Claude, ProxyService
// converts the OpenAI body to Claude format and wraps the response.
func (h *ProxyHandler) ChatCompletions(c *gin.Context) {
	if msg := checkBalance(c); msg != "" {
		h.preflightErr(c, http.StatusPaymentRequired, "insufficient_balance",
			"openai_compat", "", "balance_insufficient", msg)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"openai_compat", "", "body_read", "cannot read request body: "+err.Error())
		return
	}
	model := service.ExtractModel(body)
	if model == "" {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"openai_compat", "", "missing_model", "model field is required")
		return
	}

	pr := &service.ProxyRequest{
		UserID:        getUserID(c),
		ApiKeyID:      getApiKeyID(c),
		Model:         model,
		Body:          body,
		Stream:        service.ExtractStream(body),
		Type:          "openai_compat",
		InboundProto:  "openai",
		IP:            c.ClientIP(),
		ClientHeaders: c.Request.Header,
	}
	h.ProxyService.HandleProxy(c.Request.Context(), c.Writer, pr)
}

// ListModels handles GET /v1/models.
// Returns all enabled models in the OpenAI list format (cached 30s).
func (h *ProxyHandler) ListModels(c *gin.Context) {
	modelsCacheMu.RLock()
	if modelsCacheData != nil && time.Since(modelsCacheTime) < modelsCacheTTL {
		cfgs := modelsCacheData
		modelsCacheMu.RUnlock()
		h.writeModelsResponse(c, cfgs)
		return
	}
	modelsCacheMu.RUnlock()

	cfgs, err := h.ModelRepo.ListEnabled()
	if err != nil {
		proxyError(c, http.StatusInternalServerError, "internal_error", "failed to list models")
		return
	}

	modelsCacheMu.Lock()
	modelsCacheData = cfgs
	modelsCacheTime = time.Now()
	modelsCacheMu.Unlock()

	h.writeModelsResponse(c, cfgs)
}

func (h *ProxyHandler) writeModelsResponse(c *gin.Context, cfgs []model.ModelConfig) {

	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}

	data := make([]modelEntry, 0, len(cfgs))
	for _, cfg := range cfgs {
		data = append(data, modelEntry{
			ID:      cfg.ModelName,
			Object:  "model",
			OwnedBy: "ai-relay",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// getApiKeyID extracts the api_key_id from gin context (set by API-key auth
// middleware). Returns 0 if not present.
func getApiKeyID(c *gin.Context) int64 {
	v, _ := c.Get("api_key_id")
	id, _ := v.(int64)
	return id
}
