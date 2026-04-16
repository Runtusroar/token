package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"ai-relay/internal/config"
	"ai-relay/internal/model"
	"ai-relay/internal/pkg"
	"ai-relay/internal/repository"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// AuthService handles authentication operations.
type AuthService struct {
	UserRepo    *repository.UserRepo
	SettingRepo *repository.SettingRepo
	Config      *config.Config
}

// TokenPair holds the access and refresh tokens issued after a successful auth.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Register creates a new user account.
// It checks the register_enabled site setting, verifies the email is not already
// taken, bcrypt-hashes the password, and stores the new user with the default
// balance from config.
func (s *AuthService) Register(email, password string) (*model.User, error) {
	// Check site-wide registration toggle.
	setting, err := s.SettingRepo.Get("register_enabled")
	if err == nil && setting.Value == "false" {
		return nil, errors.New("registration is currently disabled")
	}

	// Reject duplicate email.
	existing, err := s.UserRepo.FindByEmail(email)
	if err == nil && existing != nil {
		return nil, errors.New("email already registered")
	}

	hash, err := pkg.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("register: hash password: %w", err)
	}

	user := &model.User{
		Email:        email,
		PasswordHash: hash,
		Role:         "user",
		Status:       "active",
		Balance:      s.getDefaultBalance(),
	}

	if err := s.UserRepo.Create(user); err != nil {
		return nil, fmt.Errorf("register: create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user by email and password.
// It verifies the account exists, is active, and the password is correct.
func (s *AuthService) Login(email, password string) (*model.User, error) {
	user, err := s.UserRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid email or password")
		}
		return nil, fmt.Errorf("login: find user: %w", err)
	}

	if user.Status != "active" {
		return nil, errors.New("account is disabled")
	}

	if !pkg.CheckPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid email or password")
	}

	return user, nil
}

// IssueTokens generates an access token (15 min) and a refresh token (7 days)
// for the given user.
func (s *AuthService) IssueTokens(user *model.User) (TokenPair, error) {
	accessClaims := &pkg.JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}
	accessToken, err := pkg.SignJWT(accessClaims, s.Config.JWTSecret, 15*time.Minute)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue tokens: sign access: %w", err)
	}

	refreshClaims := &pkg.JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}
	refreshToken, err := pkg.SignJWT(refreshClaims, s.Config.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return TokenPair{}, fmt.Errorf("issue tokens: sign refresh: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int((15 * time.Minute).Seconds()),
	}, nil
}

// RefreshToken verifies the given refresh JWT, looks up the user, confirms the
// account is still active, and issues a new token pair.
func (s *AuthService) RefreshToken(refreshToken string) (TokenPair, error) {
	claims, err := pkg.VerifyJWT(refreshToken, s.Config.JWTSecret)
	if err != nil {
		return TokenPair{}, errors.New("invalid or expired refresh token")
	}

	user, err := s.UserRepo.FindByID(claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TokenPair{}, errors.New("user not found")
		}
		return TokenPair{}, fmt.Errorf("refresh token: find user: %w", err)
	}

	if user.Status != "active" {
		return TokenPair{}, errors.New("account is disabled")
	}

	return s.IssueTokens(user)
}

// GoogleAuthURL builds the Google OAuth 2.0 authorization URL.
func (s *AuthService) GoogleAuthURL() string {
	params := url.Values{
		"client_id":     {s.Config.GoogleClientID},
		"redirect_uri":  {s.Config.GoogleCallback},
		"response_type": {"code"},
		"scope":         {"openid email"},
		"access_type":   {"offline"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// googleTokenResponse is the response from Google's token endpoint.
type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

// googleUserInfo holds the fields returned by Google's userinfo endpoint.
type googleUserInfo struct {
	Sub   string `json:"sub"`   // Google user ID
	Email string `json:"email"` // verified email address
}

// GoogleCallback exchanges the authorization code for a Google access token,
// fetches the user's profile, then finds or creates a local user account.
//
// Lookup order:
//  1. Find by google_id (existing OAuth user).
//  2. Find by email and link the google_id (existing password user).
//  3. Create a new user.
func (s *AuthService) GoogleCallback(code string) (*model.User, error) {
	// --- Exchange authorization code for tokens ---
	tokenResp, err := exchangeGoogleCode(code, s.Config.GoogleClientID, s.Config.GoogleSecret, s.Config.GoogleCallback)
	if err != nil {
		return nil, fmt.Errorf("google callback: exchange code: %w", err)
	}

	// --- Fetch user info from Google ---
	info, err := fetchGoogleUserInfo(tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("google callback: fetch userinfo: %w", err)
	}

	// --- Find or create local user ---
	// 1. Lookup by google_id.
	user, err := s.UserRepo.FindByGoogleID(info.Sub)
	if err == nil {
		if user.Status != "active" {
			return nil, errors.New("account is disabled")
		}
		return user, nil
	}

	// 2. Try to link to an existing email account.
	user, err = s.UserRepo.FindByEmail(info.Email)
	if err == nil {
		if user.Status != "active" {
			return nil, errors.New("account is disabled")
		}
		user.GoogleID = info.Sub
		if saveErr := s.UserRepo.Update(user); saveErr != nil {
			return nil, fmt.Errorf("google callback: link google id: %w", saveErr)
		}
		return user, nil
	}

	// 3. Create a brand-new user.
	newUser := &model.User{
		Email:    info.Email,
		GoogleID: info.Sub,
		Role:     "user",
		Status:   "active",
		Balance:  s.getDefaultBalance(),
	}
	if err := s.UserRepo.Create(newUser); err != nil {
		return nil, fmt.Errorf("google callback: create user: %w", err)
	}

	return newUser, nil
}

// exchangeGoogleCode posts to the Google token endpoint and returns the token response.
func exchangeGoogleCode(code, clientID, clientSecret, redirectURI string) (*googleTokenResponse, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return nil, fmt.Errorf("POST token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &tokenResp, nil
}

// fetchGoogleUserInfo calls the Google userinfo endpoint with the given access token.
func fetchGoogleUserInfo(accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET userinfo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	if info.Sub == "" || info.Email == "" {
		return nil, errors.New("incomplete user info from Google")
	}

	return &info, nil
}

// getDefaultBalance reads the default_balance from the settings table.
// Falls back to the environment variable config if the setting is missing.
func (s *AuthService) getDefaultBalance() decimal.Decimal {
	setting, err := s.SettingRepo.Get("default_balance")
	if err == nil && setting.Value != "" {
		if v, err := strconv.ParseFloat(setting.Value, 64); err == nil {
			return decimal.NewFromFloat(v)
		}
	}
	return decimal.NewFromFloat(s.Config.DefaultBalance)
}
