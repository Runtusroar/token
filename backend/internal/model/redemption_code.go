package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// RedemptionCode is a one-time-use code that credits a user's balance.
type RedemptionCode struct {
	ID        int64           `json:"id"         gorm:"primaryKey;autoIncrement"`
	Code      string          `json:"code"       gorm:"type:varchar(50);uniqueIndex;not null"`
	Amount    decimal.Decimal `json:"amount"     gorm:"type:decimal(12,4)"`
	Status    string          `json:"status"     gorm:"type:varchar(20);default:unused"`
	UsedBy    *int64          `json:"used_by"`
	UsedAt    *time.Time      `json:"used_at"`
	CreatedBy int64           `json:"created_by"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt *time.Time      `json:"expires_at"`
}
