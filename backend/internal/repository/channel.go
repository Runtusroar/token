package repository

import (
	"ai-relay/internal/model"
	"fmt"

	"gorm.io/gorm"
)

type ChannelRepo struct {
	DB *gorm.DB
}

func (r *ChannelRepo) Create(channel *model.Channel) error {
	return r.DB.Create(channel).Error
}

func (r *ChannelRepo) FindByID(id int64) (*model.Channel, error) {
	var channel model.Channel
	err := r.DB.First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *ChannelRepo) Update(channel *model.Channel) error {
	return r.DB.Save(channel).Error
}

func (r *ChannelRepo) Delete(id int64) error {
	return r.DB.Delete(&model.Channel{}, id).Error
}

func (r *ChannelRepo) List() ([]model.Channel, error) {
	var channels []model.Channel
	err := r.DB.Order("priority ASC, id ASC").Find(&channels).Error
	return channels, err
}

// FindActiveByModel returns active channels that support the given model name,
// ordered by priority ASC.
func (r *ChannelRepo) FindActiveByModel(modelName string) ([]model.Channel, error) {
	var channels []model.Channel
	err := r.DB.
		Where("status = 'active' AND models @> ?", fmt.Sprintf(`["%s"]`, modelName)).
		Order("priority ASC").
		Find(&channels).Error
	return channels, err
}

func (r *ChannelRepo) UpdateStatus(id int64, status string) error {
	return r.DB.Model(&model.Channel{}).
		Where("id = ?", id).
		Update("status", status).Error
}
