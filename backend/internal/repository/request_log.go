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

func (r *RequestLogRepo) Update(log *model.RequestLog) error {
	return r.DB.Save(log).Error
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
	midnight := time.Now().UTC().Truncate(24 * time.Hour)

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

// FinancialStats holds global financial metrics.
type FinancialStats struct {
	TotalConsumption decimal.Decimal // sum of cost (what users paid)
	TotalUpstream    decimal.Decimal // sum of upstream_cost (what we pay providers)
}

// FinancialStatsAll returns the total consumption and upstream cost across all successful requests.
func (r *RequestLogRepo) FinancialStatsAll() (FinancialStats, error) {
	var stats struct {
		TotalConsumption decimal.Decimal
		TotalUpstream    decimal.Decimal
	}
	err := r.DB.Model(&model.RequestLog{}).
		Select("COALESCE(SUM(cost), 0) AS total_consumption, COALESCE(SUM(upstream_cost), 0) AS total_upstream").
		Where("status = 'success'").
		Scan(&stats).Error
	return FinancialStats{
		TotalConsumption: stats.TotalConsumption,
		TotalUpstream:    stats.TotalUpstream,
	}, err
}

// UserConsumption holds aggregated usage stats for a single user.
type UserConsumption struct {
	UserID       int64           `gorm:"column:user_id"`
	RequestCount int64           `gorm:"column:request_count"`
	TotalTokens  int64           `gorm:"column:total_tokens"`
	TotalCost    decimal.Decimal `gorm:"column:total_cost"`
}

// ConsumptionByUsers returns aggregated consumption stats for the given user IDs.
func (r *RequestLogRepo) ConsumptionByUsers(userIDs []int64) (map[int64]UserConsumption, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	var results []UserConsumption
	err := r.DB.Model(&model.RequestLog{}).
		Select("user_id, COUNT(*) as request_count, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost), 0) as total_cost").
		Where("user_id IN ? AND status = 'success'", userIDs).
		Group("user_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	m := make(map[int64]UserConsumption, len(results))
	for _, c := range results {
		m[c.UserID] = c
	}
	return m, nil
}

// ApiKeyConsumption holds aggregated usage stats for a single API key.
type ApiKeyConsumption struct {
	ApiKeyID     int64           `gorm:"column:api_key_id"`
	RequestCount int64           `gorm:"column:request_count"`
	TotalTokens  int64           `gorm:"column:total_tokens"`
	TotalCost    decimal.Decimal `gorm:"column:total_cost"`
}

// ConsumptionByApiKeys returns aggregated consumption stats for the given API key IDs.
func (r *RequestLogRepo) ConsumptionByApiKeys(keyIDs []int64) (map[int64]ApiKeyConsumption, error) {
	if len(keyIDs) == 0 {
		return nil, nil
	}

	var results []ApiKeyConsumption
	err := r.DB.Model(&model.RequestLog{}).
		Select("api_key_id, COUNT(*) as request_count, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost), 0) as total_cost").
		Where("api_key_id IN ? AND status = 'success'", keyIDs).
		Group("api_key_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	m := make(map[int64]ApiKeyConsumption, len(results))
	for _, c := range results {
		m[c.ApiKeyID] = c
	}
	return m, nil
}

// DailyUserStats holds one day's stats for a single user.
type DailyUserStats struct {
	Date        string          `json:"date"         gorm:"column:date"`
	Requests    int64           `json:"requests"     gorm:"column:requests"`
	TotalTokens int64           `json:"total_tokens" gorm:"column:total_tokens"`
	Cost        decimal.Decimal `json:"cost"         gorm:"column:cost"`
}

// DailyStatsByUser returns per-day aggregated stats for a user over the last N days.
func (r *RequestLogRepo) DailyStatsByUser(userID int64, days int) ([]DailyUserStats, error) {
	var results []DailyUserStats
	err := r.DB.Model(&model.RequestLog{}).
		Select("DATE(created_at) AS date, COUNT(*) AS requests, COALESCE(SUM(total_tokens), 0) AS total_tokens, COALESCE(SUM(cost), 0) AS cost").
		Where("user_id = ? AND status = 'success' AND created_at >= CURRENT_DATE - INTERVAL '1 day' * ?", userID, days).
		Group("DATE(created_at)").
		Order("date DESC").
		Scan(&results).Error
	return results, err
}

func (r *RequestLogRepo) StatsTodayByUser(userID int64) (DailyStats, error) {
	midnight := time.Now().UTC().Truncate(24 * time.Hour)

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
