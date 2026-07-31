package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/bard/bard-backend/internal/domain"
	"github.com/bard/bard-backend/internal/email"
	"github.com/bard/bard-backend/internal/repository"
	"github.com/bard/bard-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	tokenService    *service.TokenService
	googleOAuth     *service.GoogleOAuthService
	magicLinkSvc    *service.MagicLinkService
	userRepo        *repository.UserRepository
	mailer          *email.Mailer
}

func NewAuthHandler(
	tokenService *service.TokenService,
	googleOAuth *service.GoogleOAuthService,
	magicLinkSvc *service.MagicLinkService,
	userRepo *repository.UserRepository,
	mailer *email.Mailer,
) *AuthHandler {
	return &AuthHandler{
		tokenService: tokenService,
		googleOAuth:  googleOAuth,
		magicLinkSvc: magicLinkSvc,
		userRepo:     userRepo,
		mailer:       mailer,
	}
}

// --- Request/Response structs ---

type googleAuthRequest struct {
	Code        string `json:"code" binding:"required"`
	RedirectURI string `json:"redirect_uri"`
}

type magicLinkRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type magicLinkVerifyRequest struct {
	Token string `json:"token" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type authResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *domain.User `json:"user"`
}

// GoogleAuth exchanges a Google authorization code for JWT tokens.
// POST /auth/google
func (h *AuthHandler) GoogleAuth(c *gin.Context) {
	var req googleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	// Override redirect URI if the client sends one (for multi-platform support)
	if req.RedirectURI != "" {
		h.googleOAuth.SetRedirectURL(req.RedirectURI)
	}

	// Exchange the Google auth code for user profile info
	googleUser, err := h.googleOAuth.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		log.Printf("Google ExchangeCode error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to authenticate with Google"})
		return
	}

	// Create or update the user in our database
	user, err := h.userRepo.UpsertGoogleUser(
		c.Request.Context(),
		googleUser.ID,
		googleUser.Email,
		googleUser.Name,
		googleUser.AvatarURL,
	)
	if err != nil {
		log.Printf("Google upsert error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Generate JWT tokens
	tokens, err := h.tokenService.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, authResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         user,
	})
}

// RequestMagicLink sends a magic link email to the user.
// POST /auth/magic-link/request
func (h *AuthHandler) RequestMagicLink(c *gin.Context) {
	var req magicLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}

	ctx := c.Request.Context()

	// Find or create the user
	var userID *string
	user, err := h.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}
	if user != nil {
		userID = &user.ID
	}

	// Generate and store the magic link token
	rawToken, err := h.magicLinkSvc.CreateToken(ctx, req.Email, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create magic link"})
		return
	}

	// Send the email
	if err := h.mailer.SendMagicLink(req.Email, rawToken); err != nil {
		log.Printf("Email send error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email"})
		return
	}

	// Always return 200 regardless of whether the email exists (prevents user enumeration)
	c.JSON(http.StatusOK, gin.H{"message": "if that email is registered, you will receive a sign-in link"})
}

// VerifyMagicLink verifies a magic link token and returns JWT tokens.
// POST /auth/magic-link/verify
func (h *AuthHandler) VerifyMagicLink(c *gin.Context) {
	var req magicLinkVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	ctx := c.Request.Context()

	// Verify the token (checks hash, expiry, and marks as used)
	verifiedEmail, err := h.magicLinkSvc.VerifyToken(ctx, req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired magic link"})
		return
	}

	// Find or create the user by email
	user, err := h.userRepo.GetByEmail(ctx, verifiedEmail)
	if errors.Is(err, repository.ErrUserNotFound) {
		// First time user — create their account
		user, err = h.userRepo.Create(ctx, verifiedEmail, "", domain.AuthProviderMagicLink, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
		return
	}

	// Generate JWT tokens
	tokens, err := h.tokenService.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, authResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         user,
	})
}

// RefreshToken exchanges a valid refresh token for a new token pair.
// POST /auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	// Validate the refresh token
	claims, err := h.tokenService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	// Generate a fresh token pair
	tokens, err := h.tokenService.GenerateTokenPair(claims.Subject, claims.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, service.TokenPair{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Me returns the currently authenticated user's profile.
// GET /auth/me (protected route)
func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString("user_id")

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfile lets the authenticated user update their name.
// PATCH /auth/me
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	user, err := h.userRepo.UpdateName(c.Request.Context(), userID, req.Name)
	if err != nil {
		log.Printf("Update profile error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, user)
}
