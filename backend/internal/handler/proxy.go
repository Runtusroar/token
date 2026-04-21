package handler

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ai-relay/internal/adapter"
	"ai-relay/internal/model"
	"ai-relay/internal/repository"
	"ai-relay/internal/service"
)

// responseBuffer captures an HTTP response in memory for post-processing.
type responseBuffer struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func (rb *responseBuffer) Header() http.Header {
	if rb.header == nil {
		rb.header = make(http.Header)
	}
	return rb.header
}

func (rb *responseBuffer) Write(b []byte) (int, error) {
	return rb.body.Write(b)
}

func (rb *responseBuffer) WriteHeader(code int) {
	rb.statusCode = code
}

// modelsCache caches the /v1/models response to avoid DB queries on every call.
var (
	modelsCacheMu   sync.RWMutex
	modelsCacheData []model.ModelConfig
	modelsCacheTime time.Time
	modelsCacheTTL  = 30 * time.Second
)

// checkBalance verifies the user has a positive balance before proxying.
// Returns true if the request should proceed, false if rejected.
func checkBalance(c *gin.Context) bool {
	balStr, _ := c.Get("balance")
	if s, ok := balStr.(string); ok {
		bal, err := decimal.NewFromString(s)
		if err == nil && bal.LessThanOrEqual(decimal.Zero) {
			proxyError(c, http.StatusPaymentRequired, "insufficient_balance",
				"your balance is zero or negative, please top up")
			return false
		}
	}
	return true
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

// NativeMessages handles POST /v1/messages (native Claude protocol).
// It reads the raw body, extracts model/stream, and forwards to ProxyService.
func (h *ProxyHandler) NativeMessages(c *gin.Context) {
	if !checkBalance(c) {
		return
	}

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
		UserID:        getUserID(c),
		ApiKeyID:      getApiKeyID(c),
		Model:         model,
		Body:          body,
		Stream:        stream,
		Type:          "native",
		IP:            c.ClientIP(),
		ClientHeaders: c.Request.Header,
	}

	h.ProxyService.HandleProxy(c.Request.Context(), c.Writer, pr)
}

// ChatCompletions handles POST /v1/chat/completions (OpenAI-compatible protocol).
// It converts the OpenAI request to Claude format, proxies through the adapter,
// then converts the response back to OpenAI format (both streaming and non-streaming).
func (h *ProxyHandler) ChatCompletions(c *gin.Context) {
	if !checkBalance(c) {
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		proxyError(c, http.StatusBadRequest, "invalid_request", "cannot read request body")
		return
	}

	claudeBody, reqModel, err := adapter.OpenAIToClaude(body)
	if err != nil {
		proxyError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if reqModel == "" {
		proxyError(c, http.StatusBadRequest, "invalid_request", "model field is required")
		return
	}

	stream := service.ExtractStream(body)

	pr := &service.ProxyRequest{
		UserID:        getUserID(c),
		ApiKeyID:      getApiKeyID(c),
		Model:         reqModel,
		Body:          claudeBody,
		Stream:        stream,
		Type:          "openai_compat",
		IP:            c.ClientIP(),
		ClientHeaders: c.Request.Header,
	}

	if stream {
		// Streaming: wrap the writer to convert Claude SSE → OpenAI chunks.
		includeUsage := adapter.IncludeUsageRequested(body)
		converter := adapter.NewOpenAIStreamWriter(c.Writer, reqModel, includeUsage)
		h.ProxyService.HandleProxy(c.Request.Context(), converter, pr)
	} else {
		// Non-streaming: buffer the Claude response, convert, then write.
		buf := &responseBuffer{}
		h.ProxyService.HandleProxy(c.Request.Context(), buf, pr)

		statusCode := buf.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		if statusCode == http.StatusOK {
			openaiResp, convErr := adapter.ClaudeToOpenAIResponse(buf.body.Bytes(), reqModel)
			if convErr == nil {
				c.Data(http.StatusOK, "application/json", openaiResp)
				return
			}
		}
		// Fallback: forward raw response if conversion fails or non-200.
		ct := buf.Header().Get("Content-Type")
		if ct == "" {
			ct = "application/json"
		}
		c.Data(statusCode, ct, buf.body.Bytes())
	}
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
