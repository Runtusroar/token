package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// BalanceLog tracks every credit/debit change to a user's balance.
type BalanceLog struct {
	ID           int64           `json:"id"             gorm:"primaryKey;autoIncrement"`
	UserID       int64           `json:"user_id"        gorm:"index;not null"`
	Type         string          `json:"type"           gorm:"type:varchar(20)"`
	Amount       decimal.Decimal `json:"amount"         gorm:"type:decimal(12,4)"`
	BalanceAfter decimal.Decimal `json:"balance_after"  gorm:"type:decimal(12,4)"`
	Description  string          `json:"description"    gorm:"type:varchar(500)"`
	RequestLogID *int64          `json:"request_log_id"`
	CreatedAt    time.Time       `json:"created_at"`
}
