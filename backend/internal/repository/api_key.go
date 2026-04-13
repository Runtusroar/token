package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type ApiKeyRepo struct {
	DB *gorm.DB
}

func (r *ApiKeyRepo) Create(key *model.ApiKey) error {
	return r.DB.Create(key).Error
}

func (r *ApiKeyRepo) FindByKey(key string) (*model.ApiKey, error) {
	var apiKey model.ApiKey
	err := r.DB.Where("key = ?", key).First(&apiKey).Error
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (r *ApiKeyRepo) ListByUser(userID int64) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := r.DB.Where("user_id = ?", userID).Order("id DESC").Find(&keys).Error
	return keys, err
}

func (r *ApiKeyRepo) Delete(id, userID int64) error {
	return r.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ApiKey{}).Error
}

func (r *ApiKeyRepo) UpdateStatus(id, userID int64, status string) error {
	return r.DB.Model(&model.ApiKey{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("status", status).Error
}
