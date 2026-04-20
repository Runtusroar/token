package service

import (
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"ai-relay/internal/model"
	"ai-relay/internal/repository"
)

// AdminService provides admin-only business logic operations.
type AdminService struct {
	DB             *gorm.DB
	UserRepo       *repository.UserRepo
	ChannelRepo    *repository.ChannelRepo
	ModelRepo      *repository.ModelConfigRepo
	RequestLogRepo *repository.RequestLogRepo
	SettingRepo    *repository.SettingRepo
}

// DashboardStats holds the high-level numbers shown on the admin dashboard.
type DashboardStats struct {
	// Overview
	TotalUsers    int64 `json:"total_users"`
	TodayRequests int64 `json:"today_requests"`
	TodayTokens   int64 `json:"today_tokens"`

	// Income (money in)
	RedeemIncome string `json:"redeem_income"` // sum of redeemed code amounts
	TopupIncome  string `json:"topup_income"`  // sum of admin top-ups
	TotalIncome  string `json:"total_income"`  // redeem + topup

	// User consumption (what users are charged)
	TodayConsumption string `json:"today_consumption"`
	TotalConsumption string `json:"total_consumption"`

	// Expense (what we pay upstream)
	TodayUpstream string `json:"today_upstream"`
	TotalUpstream string `json:"total_upstream"`

	// Profit
	TotalProfit string `json:"total_profit"` // consumption - upstream

	// Balance
	TotalBalance string `json:"total_balance"` // remaining user balances
}

// Dashboard returns aggregate statistics for the admin dashboard.
func (s *AdminService) Dashboard() (DashboardStats, error) {
	_, totalUsers, err := s.UserRepo.List(1, 1, "")
	if err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard: count users: %w", err)
	}

	today, err := s.RequestLogRepo.StatsToday()
	if err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard: stats today: %w", err)
	}

	fin, err := s.RequestLogRepo.FinancialStatsAll()
	if err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard: financial stats: %w", err)
	}

	// Today's upstream cost.
	var todayUpstream decimal.Decimal
	s.DB.Model(&model.RequestLog{}).
		Select("COALESCE(SUM(upstream_cost), 0)").
		Where("status = 'success' AND created_at >= (CURRENT_DATE AT TIME ZONE 'UTC')").
		Scan(&todayUpstream)

	// Income breakdown from balance_logs.
	var redeemIncome, topupIncome decimal.Decimal
	s.DB.Model(&model.BalanceLog{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("type = 'redeem'").
		Scan(&redeemIncome)
	s.DB.Model(&model.BalanceLog{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("type = 'topup'").
		Scan(&topupIncome)

	totalIncome := redeemIncome.Add(topupIncome)
	profit := fin.TotalConsumption.Sub(fin.TotalUpstream)

	var totalBalance decimal.Decimal
	s.DB.Model(&model.User{}).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&totalBalance)

	return DashboardStats{
		TotalUsers:       totalUsers,
		TodayRequests:    today.ReqCount,
		TodayTokens:      today.TotalTokens,
		RedeemIncome:     redeemIncome.String(),
		TopupIncome:      topupIncome.String(),
		TotalIncome:      totalIncome.String(),
		TodayConsumption: today.TotalCost.String(),
		TotalConsumption: fin.TotalConsumption.String(),
		TodayUpstream:    todayUpstream.String(),
		TotalUpstream:    fin.TotalUpstream.String(),
		TotalProfit:      profit.String(),
		TotalBalance:     totalBalance.String(),
	}, nil
}

// ListUsers returns a paginated, optionally filtered list of users.
func (s *AdminService) ListUsers(page, pageSize int, search string) ([]model.User, int64, error) {
	return s.UserRepo.List(page, pageSize, search)
}

// UpdateUser changes the role, status, and/or rate multiplier of a user.
// A nil rateMultiplier leaves the existing value untouched.
func (s *AdminService) UpdateUser(userID int64, role, status string, rateMultiplier *decimal.Decimal) (*model.User, error) {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("update user: find: %w", err)
	}

	if role != "" {
		user.Role = role
	}
	if status != "" {
		user.Status = status
	}
	if rateMultiplier != nil {
		user.RateMultiplier = *rateMultiplier
	}

	if err := s.UserRepo.Update(user); err != nil {
		return nil, fmt.Errorf("update user: save: %w", err)
	}

	return user, nil
}

// GetSettings returns all site-wide settings as a key→value map.
func (s *AdminService) GetSettings() (map[string]string, error) {
	settings, err := s.SettingRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	m := make(map[string]string, len(settings))
	for _, s := range settings {
		m[s.Key] = s.Value
	}
	return m, nil
}

// UpdateSetting upserts a single site-wide setting.
func (s *AdminService) UpdateSetting(key, value string) error {
	if err := s.SettingRepo.Set(key, value); err != nil {
		return fmt.Errorf("update setting %q: %w", key, err)
	}
	return nil
}
