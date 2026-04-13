package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	UserID   int64
	ApiKeyID int64
	Model    string
	Body     []byte
	Stream   bool
	Type     string // "native" or "openai_compat"
	IP       string
}

// HandleProxy selects the appropriate upstream channel, forwards the request
// via the matching adapter, then (in a background goroutine) logs the request
// and deducts the cost from the user's balance.
func (s *ProxyService) HandleProxy(w http.ResponseWriter, pr *ProxyRequest) {
	start := time.Now()

	// 1. Pick a channel.
	ch, err := s.ChannelService.SelectChannel(pr.Model)
	if err != nil {
		writeProxyError(w, http.StatusServiceUnavailable, "no_channel", err.Error())
		return
	}

	// 2. Look up the adapter for this channel type.
	adpt, ok := s.Adapters[ch.Type]
	if !ok {
		writeProxyError(w, http.StatusInternalServerError, "unsupported_channel_type",
			fmt.Sprintf("no adapter registered for channel type %q", ch.Type))
		return
	}

	// 3. Forward the request.
	result, err := adpt.ProxyRequest(w, pr.Body, pr.Model, ch.ApiKey, ch.BaseURL, pr.Stream)
	durationMs := int(time.Since(start).Milliseconds())

	status := "success"
	if err != nil || result == nil || result.StatusCode >= 400 {
		status = "error"
	}

	// 4. Async post-processing: logging + billing.
	go func() {
		promptTokens := 0
		completionTokens := 0
		resolvedModel := pr.Model

		if result != nil {
			promptTokens = result.PromptTokens
			completionTokens = result.CompletionTokens
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
			Status:           status,
			DurationMs:       durationMs,
			IPAddress:        pr.IP,
		}

		if createErr := s.RequestLogRepo.Create(logEntry); createErr != nil {
			log.Printf("proxy: log request: %v", createErr)
			return
		}

		// Deduct balance only on success.
		if status == "success" && (promptTokens+completionTokens) > 0 {
			cost, calcErr := s.BillingService.CalculateCost(
				resolvedModel,
				int64(promptTokens),
				int64(completionTokens),
			)
			if calcErr != nil {
				log.Printf("proxy: calculate cost for model %q: %v", resolvedModel, calcErr)
				return
			}

			// Update log with computed cost.
			logEntry.Cost = cost
			if updateErr := s.RequestLogRepo.Create(logEntry); updateErr != nil {
				// Not fatal — just log.
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
