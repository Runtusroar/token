package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ai-relay/internal/pkg"
	"ai-relay/internal/service"
)

// AuthHandler exposes HTTP endpoints for authentication.
type AuthHandler struct {
	AuthService *service.AuthService
}

// registerRequest is the JSON body for the Register endpoint.
type registerRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// loginRequest is the JSON body for the Login endpoint.
type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// refreshRequest is the JSON body for the Refresh endpoint.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Register handles POST /api/v1/auth/register.
// Validates the request, calls the service, and returns a 201 with a token pair.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, formatValidationError(err))
		return
	}

	user, err := h.AuthService.Register(req.Email, req.Password)
	if err != nil {
		pkg.BadRequest(c, err.Error())
		return
	}

	pair, err := h.AuthService.IssueTokens(user)
	if err != nil {
		pkg.InternalError(c, "failed to issue tokens")
		return
	}

	pkg.Created(c, pair)
}

// Login handles POST /api/v1/auth/login.
// Authenticates the user and returns a 200 with a token pair.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, formatValidationError(err))
		return
	}

	user, err := h.AuthService.Login(req.Email, req.Password)
	if err != nil {
		pkg.Unauthorized(c, err.Error())
		return
	}

	pair, err := h.AuthService.IssueTokens(user)
	if err != nil {
		pkg.InternalError(c, "failed to issue tokens")
		return
	}

	pkg.OK(c, pair)
}

// GoogleRedirect handles GET /api/v1/auth/google.
// Redirects the browser to Google's OAuth 2.0 consent screen.
func (h *AuthHandler) GoogleRedirect(c *gin.Context) {
	authURL := h.AuthService.GoogleAuthURL()
	c.Redirect(http.StatusFound, authURL)
}

// GoogleCallback handles GET /api/v1/auth/google/callback.
// Exchanges the authorization code for tokens, finds or creates the user, then
// redirects to the frontend callback URL with the token pair in the query string.
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		pkg.BadRequest(c, "missing authorization code")
		return
	}

	user, err := h.AuthService.GoogleCallback(code)
	if err != nil {
		pkg.InternalError(c, "google authentication failed")
		return
	}

	pair, err := h.AuthService.IssueTokens(user)
	if err != nil {
		pkg.InternalError(c, "failed to issue tokens")
		return
	}

	redirectURL := "/auth/callback?access_token=" + pair.AccessToken +
		"&refresh_token=" + pair.RefreshToken
	c.Redirect(http.StatusFound, redirectURL)
}

// Refresh handles POST /api/v1/auth/refresh.
// Validates the refresh token and returns a new token pair.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.BadRequest(c, "refresh_token is required")
		return
	}

	pair, err := h.AuthService.RefreshToken(req.RefreshToken)
	if err != nil {
		pkg.Unauthorized(c, err.Error())
		return
	}

	pkg.OK(c, pair)
}

// formatValidationError converts a gin binding error into a human-readable message.
func formatValidationError(err error) string {
	msg := err.Error()
	// Surface only the first validation issue for brevity.
	if idx := strings.IndexByte(msg, '\n'); idx != -1 {
		msg = msg[:idx]
	}
	return msg
}
