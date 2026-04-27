package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"ai-relay/internal/adapter"
	"ai-relay/internal/model"
	"ai-relay/internal/repository"
)

// ProxyService orchestrates request forwarding: channel selection, upstream
// proxying, usage logging, and balance deduction.
type ProxyService struct {
	ChannelService *ChannelService
	BillingService *BillingService
	RequestLogRepo *repository.RequestLogRepo
	Adapters       map[string]adapter.Adapter
}

// ProxyRequest carries all the context needed to forward a single request.
type ProxyRequest struct {
	UserID        int64
	ApiKeyID      int64
	Model         string
	Body          []byte
	Stream        bool
	Type          string // "native" or "openai_compat"
	IP            string
	ClientHeaders http.Header
}

// HandleProxy selects the appropriate upstream channel, forwards the request
// via the matching adapter, then (in a background goroutine) logs the request
// and deducts the cost from the user's balance.
func (s *ProxyService) HandleProxy(ctx context.Context, w http.ResponseWriter, pr *ProxyRequest) {
	start := time.Now()

	// 1. Pick a channel.
	ch, err := s.ChannelService.SelectChannel(pr.Model)
	if err != nil {
		writeProxyError(w, http.StatusServiceUnavailable, "no_channel", err.Error())
		s.LogPreflightError(pr, "channel_select", err.Error())
		return
	}

	// 2. Look up the adapter for this channel type.
	adpt, ok := s.Adapters[ch.Type]
	if !ok {
		msg := fmt.Sprintf("no adapter registered for channel type %q", ch.Type)
		writeProxyError(w, http.StatusInternalServerError, "unsupported_channel_type", msg)
		s.LogPreflightError(pr, "adapter_missing", msg)
		return
	}

	// 3. Forward the request.
	result, err := adpt.ProxyRequest(ctx, w, pr.Body, pr.Model, ch.ApiKey, ch.BaseURL, pr.Stream, pr.ClientHeaders)
	durationMs := int(time.Since(start).Milliseconds())

	// Classify the outcome and capture diagnostics for the log entry.
	// errorStage identifies which layer failed so the DB row is self-explanatory:
	//   upstream_transport — dial / TLS / timeout, adapter never wrote to w
	//   stream_scan        — upstream 2xx but SSE body errored mid-stream;
	//                        adapter already wrote partial response
	//   upstream_http      — upstream returned 4xx/5xx (body sampled)
	//   internal           — adapter contract violated (nil result, no err)
	status := "success"
	errorStage := ""
	upstreamStatus := 0
	upstreamError := ""
	switch {
	case err != nil && result == nil:
		// Transport-level failure; nothing written to w yet — emit a proper
		// 502 so the client sees a meaningful error instead of an empty 200.
		status = "error"
		errorStage = "upstream_transport"
		upstreamError = truncate(err.Error(), 2000)
		writeProxyError(w, http.StatusBadGateway, "upstream_transport", err.Error())
	case err != nil:
		// Partial-response error (e.g. SSE scanner bailed). Adapter already
		// wrote headers/bytes to w; do NOT write an error response on top.
		status = "error"
		errorStage = "stream_scan"
		upstreamStatus = result.StatusCode
		upstreamError = truncate(err.Error(), 2000)
	case result == nil:
		status = "error"
		errorStage = "internal"
		upstreamError = "adapter returned nil result"
		writeProxyError(w, http.StatusInternalServerError, "internal", upstreamError)
	case result.StatusCode >= 400:
		status = "error"
		errorStage = "upstream_http"
		upstreamStatus = result.StatusCode
		upstreamError = truncate(result.UpstreamError, 2000)
	default:
		upstreamStatus = result.StatusCode
	}

	// 4. Async post-processing: logging + billing.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("proxy: PANIC in billing goroutine: %v\n%s", r, debug.Stack())
			}
		}()

		promptTokens := 0
		completionTokens := 0
		cacheReadTokens := 0
		cacheWrite5m := 0
		cacheWrite1h := 0
		freshInputTokens := 0
		resolvedModel := pr.Model

		if result != nil {
			promptTokens = result.PromptTokens
			completionTokens = result.CompletionTokens
			cacheReadTokens = result.CacheReadTokens
			cacheWrite5m = result.CacheWrite5mTokens
			cacheWrite1h = result.CacheWrite1hTokens
			freshInputTokens = result.InputTokens
			if result.Model != "" {
				resolvedModel = result.Model
			}
		}

		// Create request log.
		logEntry := &model.RequestLog{
			UserID:           pr.UserID,
			ApiKeyID:         pr.ApiKeyID,
			ChannelID:        ch.ID,
			Model:            resolvedModel,
			Type:             pr.Type,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			CacheReadTokens:  cacheReadTokens,
			CacheWriteTokens: cacheWrite5m + cacheWrite1h,
			Status:           status,
			DurationMs:       durationMs,
			IPAddress:        pr.IP,
			UpstreamStatus:   upstreamStatus,
			UpstreamError:    upstreamError,
			ErrorStage:       errorStage,
		}

		if createErr := s.RequestLogRepo.Create(logEntry); createErr != nil {
			log.Printf("proxy: log request: %v", createErr)
			return
		}

		// Deduct balance only on success.
		if status == "success" && (promptTokens+completionTokens) > 0 {
			breakdown := TokenBreakdown{
				Input:        int64(freshInputTokens),
				CacheRead:    int64(cacheReadTokens),
				CacheWrite5m: int64(cacheWrite5m),
				CacheWrite1h: int64(cacheWrite1h),
				Output:       int64(completionTokens),
			}
			cost, upstreamCost, calcErr := s.BillingService.CalculateCostWithUpstream(
				pr.UserID,
				resolvedModel,
				breakdown,
			)
			if calcErr != nil {
				log.Printf("proxy: calculate cost for model %q: %v", resolvedModel, calcErr)
				return
			}

			// Update log with computed costs.
			logEntry.Cost = cost
			logEntry.UpstreamCost = upstreamCost
			if updateErr := s.RequestLogRepo.Update(logEntry); updateErr != nil {
				log.Printf("proxy: update log cost: %v", updateErr)
			}

			desc := fmt.Sprintf("API request: %s (%d+%d tokens)",
				resolvedModel, promptTokens, completionTokens)
			logID := logEntry.ID
			if deductErr := s.BillingService.DeductBalance(pr.UserID, cost, &logID, desc); deductErr != nil {
				log.Printf("proxy: deduct balance user=%d: %v", pr.UserID, deductErr)
			}
		}
	}()
}

// LogPreflightError records a request that failed before reaching the upstream
// (converter error, balance check, channel selection, adapter lookup, etc.).
// It runs asynchronously to keep the error path fast and matches the main
// logging pattern in HandleProxy.
//
// The caller should have already written the error response to the client —
// this only persists a row to request_logs so support / the admin UI can see
// what happened. user_id / api_key_id may be 0 when the middleware chain
// rejected the request before authenticating (e.g. no key).
func (s *ProxyService) LogPreflightError(pr *ProxyRequest, stage, message string) {
	if s == nil || s.RequestLogRepo == nil {
		return
	}
	entry := &model.RequestLog{
		UserID:        pr.UserID,
		ApiKeyID:      pr.ApiKeyID,
		Model:         pr.Model,
		Type:          pr.Type,
		Status:        "error",
		IPAddress:     pr.IP,
		ErrorStage:    stage,
		UpstreamError: truncate(message, 2000),
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("proxy: PANIC in preflight log: %v\n%s", r, debug.Stack())
			}
		}()
		if err := s.RequestLogRepo.Create(entry); err != nil {
			log.Printf("proxy: preflight log: %v", err)
		}
	}()
}

// ExtractModel reads the "model" field from a JSON body without full
// unmarshalling. Returns an empty string if the field is absent or malformed.
func ExtractModel(body []byte) string {
	var v struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &v)
	return v.Model
}

// ExtractStream reads the "stream" boolean from a JSON body.
func ExtractStream(body []byte) bool {
	var v struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &v)
	return v.Stream
}

// truncate clamps s to at most max bytes, appending "…" when it had to cut.
// Operates on bytes (not runes) to keep the DB column bounded predictably.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// writeProxyError writes an Anthropic-style error JSON response.
func writeProxyError(w http.ResponseWriter, statusCode int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	payload := map[string]any{
		"error": map[string]string{
			"type":    errType,
			"message": message,
		},
	}
	_ = json.NewEncoder(w).Encode(payload)
}
