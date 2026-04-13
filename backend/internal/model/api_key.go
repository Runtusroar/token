package model

import "time"

// ApiKey represents a relay API key belonging to a user.
type ApiKey struct {
	ID         int64      `json:"id"          gorm:"primaryKey;autoIncrement"`
	UserID     int64      `json:"user_id"     gorm:"index;not null"`
	Key        string     `json:"-"           gorm:"type:varchar(255);uniqueIndex;not null"`
	Name       string     `json:"name"        gorm:"type:varchar(100)"`
	Status     string     `json:"status"      gorm:"type:varchar(20);default:active"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`

	// Relations
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}
