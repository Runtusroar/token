package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// ModelConfig stores pricing and display configuration for an AI model.
type ModelConfig struct {
	ID           int64           `json:"id"            gorm:"primaryKey;autoIncrement"`
	ModelName    string          `json:"model_name"    gorm:"type:varchar(100);uniqueIndex;not null"`
	DisplayName  string          `json:"display_name"  gorm:"type:varchar(100)"`
	Rate         decimal.Decimal `json:"rate"          gorm:"type:decimal(6,2);default:1.00"`
	InputPrice   decimal.Decimal `json:"input_price"   gorm:"type:decimal(10,6)"`
	OutputPrice  decimal.Decimal `json:"output_price"  gorm:"type:decimal(10,6)"`
	Enabled      bool            `json:"enabled"       gorm:"default:true"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
