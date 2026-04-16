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
	Cost             decimal.Decimal `json:"cost"              gorm:"type:decimal(10,6)"`
	UpstreamCost     decimal.Decimal `json:"upstream_cost"     gorm:"type:decimal(10,6)"`
	Status           string          `json:"status"            gorm:"type:varchar(20)"`
	DurationMs       int             `json:"duration_ms"`
	IPAddress        string          `json:"ip_address"        gorm:"type:varchar(45)"`
	CreatedAt        time.Time       `json:"created_at"        gorm:"index"`
}
