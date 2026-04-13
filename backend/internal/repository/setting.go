package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingRepo struct {
	DB *gorm.DB
}

func (r *SettingRepo) Get(key string) (*model.Setting, error) {
	var setting model.Setting
	err := r.DB.First(&setting, "key = ?", key).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// Set upserts the setting value for the given key.
func (r *SettingRepo) Set(key, value string) error {
	setting := model.Setting{Key: key, Value: value}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&setting).Error
}

func (r *SettingRepo) GetAll() ([]model.Setting, error) {
	var settings []model.Setting
	err := r.DB.Find(&settings).Error
	return settings, err
}
