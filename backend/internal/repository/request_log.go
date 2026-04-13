package repository

import (
	"ai-relay/internal/model"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type RequestLogRepo struct {
	DB *gorm.DB
}

func (r *RequestLogRepo) Create(log *model.RequestLog) error {
	return r.DB.Create(log).Error
}

func (r *RequestLogRepo) ListByUser(userID int64, page, pageSize int) ([]model.RequestLog, int64, error) {
	var logs []model.RequestLog
	var total int64

	q := r.DB.Model(&model.RequestLog{}).Where("user_id = ?", userID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *RequestLogRepo) ListAll(page, pageSize int, userID int64, modelFilter string) ([]model.RequestLog, int64, error) {
	var logs []model.RequestLog
	var total int64

	q := r.DB.Model(&model.RequestLog{})
	if userID != 0 {
		q = q.Where("user_id = ?", userID)
	}
	if modelFilter != "" {
		q = q.Where("model = ?", modelFilter)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

type DailyStats struct {
	ReqCount    int64
	TotalTokens int64
	TotalCost   decimal.Decimal
}

func (r *RequestLogRepo) StatsToday() (DailyStats, error) {
	midnight := time.Now().Truncate(24 * time.Hour)

	var stats struct {
		ReqCount    int64
		TotalTokens int64
		TotalCost   decimal.Decimal
	}

	err := r.DB.Model(&model.RequestLog{}).
		Select("COUNT(*) AS req_count, SUM(total_tokens) AS total_tokens, SUM(cost) AS total_cost").
		Where("created_at >= ? AND status = 'success'", midnight).
		Scan(&stats).Error

	return DailyStats{
		ReqCount:    stats.ReqCount,
		TotalTokens: stats.TotalTokens,
		TotalCost:   stats.TotalCost,
	}, err
}

func (r *RequestLogRepo) StatsTodayByUser(userID int64) (DailyStats, error) {
	midnight := time.Now().Truncate(24 * time.Hour)

	var stats struct {
		ReqCount    int64
		TotalTokens int64
		TotalCost   decimal.Decimal
	}

	err := r.DB.Model(&model.RequestLog{}).
		Select("COUNT(*) AS req_count, SUM(total_tokens) AS total_tokens, SUM(cost) AS total_cost").
		Where("user_id = ? AND created_at >= ? AND status = 'success'", userID, midnight).
		Scan(&stats).Error

	return DailyStats{
		ReqCount:    stats.ReqCount,
		TotalTokens: stats.TotalTokens,
		TotalCost:   stats.TotalCost,
	}, err
}
