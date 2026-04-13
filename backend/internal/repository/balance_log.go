package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type BalanceLogRepo struct {
	DB *gorm.DB
}

func (r *BalanceLogRepo) Create(log *model.BalanceLog) error {
	return r.DB.Create(log).Error
}

func (r *BalanceLogRepo) ListByUser(userID int64, page, pageSize int) ([]model.BalanceLog, int64, error) {
	var logs []model.BalanceLog
	var total int64

	q := r.DB.Model(&model.BalanceLog{}).Where("user_id = ?", userID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
