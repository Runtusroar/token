package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type ModelConfigRepo struct {
	DB *gorm.DB
}

func (r *ModelConfigRepo) Create(cfg *model.ModelConfig) error {
	return r.DB.Create(cfg).Error
}

func (r *ModelConfigRepo) FindByID(id int64) (*model.ModelConfig, error) {
	var cfg model.ModelConfig
	err := r.DB.First(&cfg, id).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ModelConfigRepo) FindByName(modelName string) (*model.ModelConfig, error) {
	var cfg model.ModelConfig
	err := r.DB.Where("model_name = ?", modelName).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ModelConfigRepo) Update(cfg *model.ModelConfig) error {
	return r.DB.Save(cfg).Error
}

func (r *ModelConfigRepo) Delete(id int64) error {
	return r.DB.Delete(&model.ModelConfig{}, id).Error
}

func (r *ModelConfigRepo) List() ([]model.ModelConfig, error) {
	var cfgs []model.ModelConfig
	err := r.DB.Order("id ASC").Find(&cfgs).Error
	return cfgs, err
}

func (r *ModelConfigRepo) ListEnabled() ([]model.ModelConfig, error) {
	var cfgs []model.ModelConfig
	err := r.DB.Where("enabled = true").Order("id ASC").Find(&cfgs).Error
	return cfgs, err
}
