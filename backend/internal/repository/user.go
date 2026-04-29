package repository

import (
	"ai-relay/internal/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type UserRepo struct {
	DB *gorm.DB
}

func (r *UserRepo) Create(user *model.User) error {
	return r.DB.Create(user).Error
}

func (r *UserRepo) FindByID(id int64) (*model.User, error) {
	var user model.User
	err := r.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) FindByGoogleID(googleID string) (*model.User, error) {
	var user model.User
	err := r.DB.Where("google_id = ?", googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) Update(user *model.User) error {
	return r.DB.Save(user).Error
}

func (r *UserRepo) List(page, pageSize int, search string) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	q := r.DB.Model(&model.User{})
	if search != "" {
		q = q.Where("email ILIKE ?", "%"+search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// DeductBalance subtracts amount from the user's balance unconditionally.
// We intentionally allow the balance to go negative on the final overdrafting
// request so users can't farm free responses by holding a tiny positive
// balance — the preflight checkBalance gate (balance > 0) then locks the
// account out until they top up. Returns rowsAffected (0 means user not found).
func (r *UserRepo) DeductBalance(userID int64, amount decimal.Decimal) (int64, error) {
	result := r.DB.Exec(
		"UPDATE users SET balance = balance - ? WHERE id = ?",
		amount, userID,
	)
	return result.RowsAffected, result.Error
}

func (r *UserRepo) AddBalance(userID int64, amount decimal.Decimal) error {
	return r.DB.Exec(
		"UPDATE users SET balance = balance + ? WHERE id = ?",
		amount, userID,
	).Error
}
