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

// Anthropic prompt-caching pricing multipliers, applied on top of a model's
// base input price. Source:
// https://platform.claude.com/docs/zh-CN/about-claude/pricing#prompt-caching
//
// These are protocol-level constants — uniform across Opus/Sonnet/Haiku and
// stable since the cache feature launched. If a future upstream (Bedrock,
// Vertex, …) or a business decision requires per-model overrides, move them
// to model_configs columns.
var (
	cacheReadMultiplier    = decimal.NewFromFloat(0.1)
	cacheWrite5mMultiplier = decimal.NewFromFloat(1.25)
	cacheWrite1hMultiplier = decimal.NewFromFloat(2.0)
	million                = decimal.NewFromInt(1_000_000)
)

// TokenBreakdown carries the four input categories Anthropic charges
// separately, plus output. Counts come straight from the upstream usage
// object; billing applies cache multipliers internally.
type TokenBreakdown struct {
	Input        int64 // fresh input (uncached)
	CacheRead    int64 // cache_read_input_tokens
	CacheWrite5m int64 // cache_creation, 5-minute ttl
	CacheWrite1h int64 // cache_creation, 1-hour ttl
	Output       int64 // output_tokens
}

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

// rawCost computes the upstream-equivalent cost (no markup, no userRate)
// using Anthropic's per-class multipliers on the configured base prices:
//
//	raw = (input × 1.0 + cacheRead × 0.1 + cw5m × 1.25 + cw1h × 2.0) × inputPrice
//	    + output × outputPrice
//	    , then ÷ 1_000_000.
//
// Both CalculateCost variants delegate here, then layer rate × userRate on top.
func rawCost(cfg *model.ModelConfig, t TokenBreakdown) decimal.Decimal {
	inputUnits := decimal.NewFromInt(t.Input).
		Add(decimal.NewFromInt(t.CacheRead).Mul(cacheReadMultiplier)).
		Add(decimal.NewFromInt(t.CacheWrite5m).Mul(cacheWrite5mMultiplier)).
		Add(decimal.NewFromInt(t.CacheWrite1h).Mul(cacheWrite1hMultiplier))
	inputCost := inputUnits.Mul(cfg.InputPrice)
	outputCost := decimal.NewFromInt(t.Output).Mul(cfg.OutputPrice)
	return inputCost.Add(outputCost).Div(million)
}

// CalculateCost computes the user-facing cost of a request, applying the
// per-model rate and the user's personal rate multiplier on top of the
// upstream raw cost.
func (s *BillingService) CalculateCost(userID int64, modelName string, t TokenBreakdown) (decimal.Decimal, error) {
	cfg, err := s.ModelRepo.FindByName(modelName)
	if err != nil {
		return decimal.Zero, err
	}
	userRate, err := s.UserRateMultiplier(userID)
	if err != nil {
		return decimal.Zero, err
	}
	return rawCost(cfg, t).Mul(cfg.Rate).Mul(userRate), nil
}

// CalculateCostWithUpstream returns both the user-facing cost (with model
// rate and user multiplier applied) and the upstream raw cost (what we
// approximately pay the provider) for a request.
func (s *BillingService) CalculateCostWithUpstream(userID int64, modelName string, t TokenBreakdown) (cost, upstreamCost decimal.Decimal, err error) {
	cfg, err := s.ModelRepo.FindByName(modelName)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	userRate, err := s.UserRateMultiplier(userID)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	raw := rawCost(cfg, t)
	return raw.Mul(cfg.Rate).Mul(userRate), raw, nil
}

// UserRateMultiplier fetches the user's billing rate multiplier. Returns 1.00
// if the stored multiplier is zero or negative so a misconfigured user never
// gets free usage silently.
func (s *BillingService) UserRateMultiplier(userID int64) (decimal.Decimal, error) {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return decimal.Zero, err
	}
	if user.RateMultiplier.LessThanOrEqual(decimal.Zero) {
		return decimal.NewFromInt(1), nil
	}
	return user.RateMultiplier, nil
}

// DeductBalance atomically deducts cost from the user's balance inside a
// transaction and appends a balance_log entry. The deduction is unconditional
// (balance may go negative on the final overdrafting request); the preflight
// balance > 0 gate then prevents further usage until the user tops up.
func (s *BillingService) DeductBalance(userID int64, cost decimal.Decimal, requestLogID *int64, description string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := &repository.UserRepo{DB: tx}
		balanceLogRepo := &repository.BalanceLogRepo{DB: tx}

		rows, err := userRepo.DeductBalance(userID, cost)
		if err != nil {
			return err
		}
		if rows == 0 {
			return errors.New("user not found")
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

// AdminDeduct subtracts the given amount from a user's balance inside a
// transaction and records a balance_log entry. Allows the balance to go
// negative — admin override. Reason (optional) is appended to the
// description for audit visibility.
func (s *BillingService) AdminDeduct(userID int64, amount decimal.Decimal, reason string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := &repository.UserRepo{DB: tx}
		balanceLogRepo := &repository.BalanceLogRepo{DB: tx}

		rows, err := userRepo.DeductBalance(userID, amount)
		if err != nil {
			return err
		}
		if rows == 0 {
			return errors.New("user not found")
		}

		var user model.User
		if err := tx.Select("balance").First(&user, userID).Error; err != nil {
			return err
		}

		desc := "Admin deduction"
		if reason != "" {
			desc = "Admin deduction: " + reason
		}
		entry := &model.BalanceLog{
			UserID:       userID,
			Type:         "admin_deduct",
			Amount:       amount.Neg(),
			BalanceAfter: user.Balance,
			Description:  desc,
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
