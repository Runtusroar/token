package service

import (
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

// ApiKeyService handles business logic for user API keys.
type ApiKeyService struct {
	Repo *repository.ApiKeyRepo
}

// Create generates a new sk- prefixed API key and persists it.
func (s *ApiKeyService) Create(userID int64, name string) (*model.ApiKey, error) {
	rawKey, err := pkg.GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	key := &model.ApiKey{
		UserID: userID,
		Key:    rawKey,
		Name:   name,
		Status: "active",
	}

	if err := s.Repo.Create(key); err != nil {
		return nil, err
	}

	return key, nil
}

// List returns all API keys belonging to the given user.
func (s *ApiKeyService) List(userID int64) ([]model.ApiKey, error) {
	return s.Repo.ListByUser(userID)
}

// Delete removes an API key, verifying it belongs to the given user.
func (s *ApiKeyService) Delete(id, userID int64) error {
	return s.Repo.Delete(id, userID)
}

// UpdateStatus sets the status (active/inactive) of an API key owned by the user.
func (s *ApiKeyService) UpdateStatus(id, userID int64, status string) error {
	return s.Repo.UpdateStatus(id, userID, status)
}
