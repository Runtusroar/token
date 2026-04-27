package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// RequestLog records each proxied API request.
type RequestLog struct {
	ID               int64           `json:"id"                gorm:"primaryKey;autoIncrement"`
	UserID           int64           `json:"user_id"           gorm:"index"`
	ApiKeyID         int64           `json:"api_key_id"           gorm:"index"`
	ChannelID        int64           `json:"channel_id"        gorm:"index"`
	Model            string          `json:"model"             gorm:"type:varchar(100)"`
	Type             string          `json:"type"              gorm:"type:varchar(20)"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens"`
	// Cache audit fields. CacheReadTokens counts cache_read_input_tokens
	// (billed at 0.1× base input). CacheWriteTokens sums 5m + 1h
	// cache_creation tokens (5m at 1.25×, 1h at 2× base input). Both feed
	// into PromptTokens already; these columns let admins inspect cache
	// hit rate and split. Default 0 keeps old rows readable.
	CacheReadTokens  int             `json:"cache_read_tokens"  gorm:"default:0"`
	CacheWriteTokens int             `json:"cache_write_tokens" gorm:"default:0"`
	Cost             decimal.Decimal `json:"cost"              gorm:"type:decimal(10,6)"`
	UpstreamCost     decimal.Decimal `json:"upstream_cost"     gorm:"type:decimal(10,6)"`
	Status           string          `json:"status"            gorm:"type:varchar(20)"`
	DurationMs       int             `json:"duration_ms"`
	IPAddress        string          `json:"ip_address"        gorm:"type:varchar(45)"`
	// Error diagnostics (populated only when status="error").
	// UpstreamStatus is the HTTP status returned by the upstream channel
	// (0 if the request failed before reaching the upstream, e.g. transport
	// error or adapter pre-flight).
	// UpstreamError holds a sampled upstream response body (~2KB) or a
	// transport-error string — whatever best identifies the failure source.
	// ErrorStage says which layer failed (see service/proxy.go).
	UpstreamStatus   int             `json:"upstream_status"   gorm:"default:0"`
	UpstreamError    string          `json:"upstream_error"    gorm:"type:varchar(2048)"`
	ErrorStage       string          `json:"error_stage"       gorm:"type:varchar(50)"`
	CreatedAt        time.Time       `json:"created_at"        gorm:"index"`
}
