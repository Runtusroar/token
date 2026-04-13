package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"ai-relay/internal/adapter"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

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

// NativeMessages handles POST /v1/messages (native Claude protocol).
// It reads the raw body, extracts model/stream, and forwards to ProxyService.
func (h *ProxyHandler) NativeMessages(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		proxyError(c, http.StatusBadRequest, "invalid_request", "cannot read request body")
		return
	}

	model := service.ExtractModel(body)
	if model == "" {
		proxyError(c, http.StatusBadRequest, "invalid_request", "model field is required")
		return
	}

	stream := service.ExtractStream(body)

	pr := &service.ProxyRequest{
		UserID:   getUserID(c),
		ApiKeyID: getApiKeyID(c),
		Model:    model,
		Body:     body,
		Stream:   stream,
		Type:     "native",
		IP:       c.ClientIP(),
	}

	h.ProxyService.HandleProxy(c.Writer, pr)
}

// ChatCompletions handles POST /v1/chat/completions (OpenAI-compatible protocol).
// It converts the OpenAI request to Claude format, then forwards to ProxyService.
func (h *ProxyHandler) ChatCompletions(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		proxyError(c, http.StatusBadRequest, "invalid_request", "cannot read request body")
		return
	}

	claudeBody, model, err := adapter.OpenAIToClaude(body)
	if err != nil {
		proxyError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if model == "" {
		proxyError(c, http.StatusBadRequest, "invalid_request", "model field is required")
		return
	}

	stream := service.ExtractStream(body)

	pr := &service.ProxyRequest{
		UserID:   getUserID(c),
		ApiKeyID: getApiKeyID(c),
		Model:    model,
		Body:     claudeBody,
		Stream:   stream,
		Type:     "openai_compat",
		IP:       c.ClientIP(),
	}

	h.ProxyService.HandleProxy(c.Writer, pr)
}

// ListModels handles GET /v1/models.
// Returns all enabled models in the OpenAI list format.
func (h *ProxyHandler) ListModels(c *gin.Context) {
	cfgs, err := h.ModelRepo.ListEnabled()
	if err != nil {
		proxyError(c, http.StatusInternalServerError, "internal_error", "failed to list models")
		return
	}

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
