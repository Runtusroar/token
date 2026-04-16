package service

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"
)

// BillingService handles all money-related operations: cost calculation,
// balance deduction, top-ups, and redemption codes.
type BillingService struct {
	DB             *gorm.DB
	UserRepo       *repository.UserRepo
	BalanceLogRepo *repository.BalanceLogRepo
	RedeemRepo     *repository.RedemptionCodeRepo
	ModelRepo      *repository.ModelConfigRepo
	RequestLogRepo *repository.RequestLogRepo
}

// CalculateCost computes the cost of a request for the given model and token counts.
// Cost = (inputPrice * promptTokens + outputPrice * completionTokens) / 1_000_000 * rate
func (s *BillingService) CalculateCost(modelName string, promptTokens, completionTokens int64) (decimal.Decimal, error) {
	cfg, err := s.ModelRepo.FindByName(modelName)
	if err != nil {
		return decimal.Zero, err
	}

	million := decimal.NewFromInt(1_000_000)
	inputCost := cfg.InputPrice.Mul(decimal.NewFromInt(promptTokens))
	outputCost := cfg.OutputPrice.Mul(decimal.NewFromInt(completionTokens))
	cost := inputCost.Add(outputCost).Div(million).Mul(cfg.Rate)
	return cost, nil
}

// CalculateCostWithUpstream returns both the user-facing cost (with rate markup)
// and the upstream cost (without markup) for a request.
func (s *BillingService) CalculateCostWithUpstream(modelName string, promptTokens, completionTokens int64) (cost, upstreamCost decimal.Decimal, err error) {
	cfg, err := s.ModelRepo.FindByName(modelName)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	million := decimal.NewFromInt(1_000_000)
	inputCost := cfg.InputPrice.Mul(decimal.NewFromInt(promptTokens))
	outputCost := cfg.OutputPrice.Mul(decimal.NewFromInt(completionTokens))
	raw := inputCost.Add(outputCost).Div(million)
	return raw.Mul(cfg.Rate), raw, nil
}

// DeductBalance atomically deducts cost from the user's balance inside a
// transaction and appends a balance_log entry. Returns an error if the
// user's balance is insufficient.
func (s *BillingService) DeductBalance(userID int64, cost decimal.Decimal, requestLogID *int64, description string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := &repository.UserRepo{DB: tx}
		balanceLogRepo := &repository.BalanceLogRepo{DB: tx}

		rows, err := userRepo.DeductBalance(userID, cost)
		if err != nil {
			return err
		}
		if rows == 0 {
			return errors.New("insufficient balance")
		}

		// Read balance within the same transaction for consistency.
		var user model.User
		if err := tx.Select("balance").First(&user, userID).Error; err != nil {
			return err
		}

		entry := &model.BalanceLog{
			UserID:       userID,
			Type:         "deduct",
			Amount:       cost.Neg(),
			BalanceAfter: user.Balance,
			Description:  description,
			RequestLogID: requestLogID,
		}
		return balanceLogRepo.Create(entry)
	})
}

// AdminTopUp adds the given amount to a user's balance inside a transaction
// and records a balance_log entry.
func (s *BillingService) AdminTopUp(userID int64, amount decimal.Decimal, adminID int64) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := &repository.UserRepo{DB: tx}
		balanceLogRepo := &repository.BalanceLogRepo{DB: tx}

		if err := userRepo.AddBalance(userID, amount); err != nil {
			return err
		}

		var user model.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		entry := &model.BalanceLog{
			UserID:      userID,
			Type:        "topup",
			Amount:      amount,
			BalanceAfter: user.Balance,
			Description: "Admin top-up",
		}
		return balanceLogRepo.Create(entry)
	})
}

// Redeem validates a redemption code and credits the user's balance.
// It checks that the code exists, is unused, and has not expired.
func (s *BillingService) Redeem(userID int64, code string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		redeemRepo := &repository.RedemptionCodeRepo{DB: tx}
		userRepo := &repository.UserRepo{DB: tx}
		balanceLogRepo := &repository.BalanceLogRepo{DB: tx}

		rc, err := redeemRepo.FindByCode(code)
		if err != nil {
			return errors.New("invalid redemption code")
		}

		if rc.Status != "unused" {
			return errors.New("redemption code already used")
		}

		if rc.ExpiresAt != nil && rc.ExpiresAt.Before(time.Now()) {
			return errors.New("redemption code has expired")
		}

		// Mark as used.
		now := time.Now()
		rc.Status = "used"
		rc.UsedBy = &userID
		rc.UsedAt = &now
		if err := redeemRepo.Update(rc); err != nil {
			return err
		}

		// Credit the user.
		if err := userRepo.AddBalance(userID, rc.Amount); err != nil {
			return err
		}

		var user model.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		entry := &model.BalanceLog{
			UserID:      userID,
			Type:        "redeem",
			Amount:      rc.Amount,
			BalanceAfter: user.Balance,
			Description: "Redeemed code: " + code,
		}
		return balanceLogRepo.Create(entry)
	})
}

// GenerateRedeemCodes batch-creates redemption codes for the given amount value.
func (s *BillingService) GenerateRedeemCodes(adminID int64, amount decimal.Decimal, count int, expiresAt *time.Time) ([]model.RedemptionCode, error) {
	codes := make([]model.RedemptionCode, 0, count)
	for i := 0; i < count; i++ {
		code := pkg.GenerateRedeemCode()
		rc := model.RedemptionCode{
			Code:      code,
			Amount:    amount,
			Status:    "unused",
			CreatedBy: adminID,
			ExpiresAt: expiresAt,
		}
		if err := s.RedeemRepo.Create(&rc); err != nil {
			return nil, err
		}
		codes = append(codes, rc)
	}
	return codes, nil
}
