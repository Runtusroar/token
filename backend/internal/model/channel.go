package model

import (
	"time"

	"gorm.io/datatypes"
)

// Channel represents an upstream AI provider channel.
type Channel struct {
	ID        int64          `json:"id"         gorm:"primaryKey;autoIncrement"`
	Name      string         `json:"name"       gorm:"type:varchar(100)"`
	Type      string         `json:"type"       gorm:"type:varchar(50)"`
	ApiKey    string         `json:"-"          gorm:"type:varchar(500)"`
	BaseURL   string         `json:"base_url"   gorm:"type:varchar(500)"`
	Models    datatypes.JSON `json:"models"     gorm:"type:jsonb"`
	Status    string         `json:"status"     gorm:"type:varchar(20);default:active"`
	Priority  int            `json:"priority"   gorm:"default:0"`
	Weight    int            `json:"weight"     gorm:"default:1"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
