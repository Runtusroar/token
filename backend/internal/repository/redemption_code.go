package repository

import (
	"ai-relay/internal/model"

	"gorm.io/gorm"
)

type RedemptionCodeRepo struct {
	DB *gorm.DB
}

func (r *RedemptionCodeRepo) Create(code *model.RedemptionCode) error {
	return r.DB.Create(code).Error
}

func (r *RedemptionCodeRepo) FindByCode(code string) (*model.RedemptionCode, error) {
	var rc model.RedemptionCode
	err := r.DB.Where("code = ?", code).First(&rc).Error
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

func (r *RedemptionCodeRepo) Update(code *model.RedemptionCode) error {
	return r.DB.Save(code).Error
}

func (r *RedemptionCodeRepo) List(page, pageSize int) ([]model.RedemptionCode, int64, error) {
	var codes []model.RedemptionCode
	var total int64

	q := r.DB.Model(&model.RedemptionCode{})

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&codes).Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}
