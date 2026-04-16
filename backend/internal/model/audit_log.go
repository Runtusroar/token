package model

import "time"

// AuditLog records admin actions for accountability and compliance.
type AuditLog struct {
	ID        int64     `json:"id"         gorm:"primaryKey;autoIncrement"`
	AdminID   int64     `json:"admin_id"   gorm:"index"`
	Action    string    `json:"action"     gorm:"type:varchar(50)"`
	Target    string    `json:"target"     gorm:"type:varchar(100)"`
	TargetID  int64     `json:"target_id"`
	Detail    string    `json:"detail"     gorm:"type:text"`
	IP        string    `json:"ip"         gorm:"type:varchar(45)"`
	CreatedAt time.Time `json:"created_at"`
}
