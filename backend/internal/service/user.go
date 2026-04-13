package service

import (
	"errors"

	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

// UserService handles business logic for user profile operations.
type UserService struct {
	UserRepo       *repository.UserRepo
	RequestLogRepo *repository.RequestLogRepo
}

// GetProfile retrieves a user's profile by ID.
func (s *UserService) GetProfile(userID int64) (*model.User, error) {
	return s.UserRepo.FindByID(userID)
}

// ChangePassword updates the user's password after verifying the old one.
// If the user has no password hash set (OAuth-only account) the old password
// check is skipped so the user can set an initial password.
func (s *UserService) ChangePassword(userID int64, oldPass, newPass string) error {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return err
	}

	// If the user already has a password, verify the old one first.
	if user.PasswordHash != "" {
		if !pkg.CheckPassword(oldPass, user.PasswordHash) {
			return errors.New("incorrect old password")
		}
	}

	hash, err := pkg.HashPassword(newPass)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	return s.UserRepo.Update(user)
}

// DashboardData bundles a user's balance with today's request statistics.
type DashboardData struct {
	Balance     interface{}              `json:"balance"`
	TodayStats  repository.DailyStats   `json:"today_stats"`
}

// Dashboard returns the user's current balance and today's usage stats.
func (s *UserService) Dashboard(userID int64) (*DashboardData, error) {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.RequestLogRepo.StatsTodayByUser(userID)
	if err != nil {
		return nil, err
	}

	return &DashboardData{
		Balance:    user.Balance,
		TodayStats: stats,
	}, nil
}
