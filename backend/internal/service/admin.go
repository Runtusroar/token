package service

import (
	"fmt"

	"ai-relay/internal/model"
	"ai-relay/internal/repository"
)

// AdminService provides admin-only business logic operations.
type AdminService struct {
	UserRepo       *repository.UserRepo
	ChannelRepo    *repository.ChannelRepo
	ModelRepo      *repository.ModelConfigRepo
	RequestLogRepo *repository.RequestLogRepo
	SettingRepo    *repository.SettingRepo
}

// DashboardStats holds the high-level numbers shown on the admin dashboard.
type DashboardStats struct {
	TotalUsers    int64  `json:"total_users"`
	TodayRequests int64  `json:"today_requests"`
	TodayTokens   int64  `json:"today_tokens"`
	TodayRevenue  string `json:"today_revenue"`
}

// Dashboard returns aggregate statistics for the admin dashboard.
func (s *AdminService) Dashboard() (DashboardStats, error) {
	// Total registered users.
	_, totalUsers, err := s.UserRepo.List(1, 1, "")
	if err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard: count users: %w", err)
	}

	// Today's request / token / revenue stats.
	stats, err := s.RequestLogRepo.StatsToday()
	if err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard: stats today: %w", err)
	}

	return DashboardStats{
		TotalUsers:    totalUsers,
		TodayRequests: stats.ReqCount,
		TodayTokens:   stats.TotalTokens,
		TodayRevenue:  stats.TotalCost.String(),
	}, nil
}

// ListUsers returns a paginated, optionally filtered list of users.
func (s *AdminService) ListUsers(page, pageSize int, search string) ([]model.User, int64, error) {
	return s.UserRepo.List(page, pageSize, search)
}

// UpdateUser changes the role and/or status of a user.
func (s *AdminService) UpdateUser(userID int64, role, status string) (*model.User, error) {
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
