package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// User represents an application user account.
type User struct {
	ID             int64           `json:"id"              gorm:"primaryKey;autoIncrement"`
	Email          string          `json:"email"           gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash   string          `json:"-"               gorm:"type:varchar(255)"`
	GoogleID       string          `json:"google_id"       gorm:"type:varchar(255);index"`
	Role           string          `json:"role"            gorm:"type:varchar(20);default:user"`
	Balance        decimal.Decimal `json:"balance"         gorm:"type:decimal(12,4);default:0"`
	RateMultiplier decimal.Decimal `json:"rate_multiplier" gorm:"type:decimal(6,2);default:1.00;not null"`
	Status         string          `json:"status"          gorm:"type:varchar(20);default:active"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
