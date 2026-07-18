package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

// Claims is the data we embed inside the JWT token
type Claims struct {
	Email string `json:"email"`
	Type  string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// TokenPair holds both tokens returned to the client after login
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// TokenService handles JWT creation and validation
type TokenService struct {
	accessSecret  []byte
	refreshSecret []byte
}

// NewTokenService creates a new TokenService with the signing secrets
func NewTokenService(accessSecret, refreshSecret string) *TokenService {
	return &TokenService{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
	}
}

// GenerateTokenPair creates both an access token (short-lived) and refresh token (long-lived)
func (s *TokenService) GenerateTokenPair(userID, email string) (*TokenPair, error) {
	accessToken, err := s.generateToken(userID, email, "access", AccessTokenDuration, s.accessSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateToken(userID, email, "refresh", RefreshTokenDuration, s.refreshSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateAccessToken checks if an access token is valid and returns the claims
func (s *TokenService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return s.validateToken(tokenStr, s.accessSecret, "access")
}

// ValidateRefreshToken checks if a refresh token is valid and returns the claims
func (s *TokenService) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return s.validateToken(tokenStr, s.refreshSecret, "refresh")
}

func (s *TokenService) generateToken(userID, email, tokenType string, duration time.Duration, secret []byte) (string, error) {
	now := time.Now()

	claims := &Claims{
		Email: email,
		Type:  tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func (s *TokenService) validateToken(tokenStr string, secret []byte, expectedType string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is what we expect (prevents algorithm swap attacks)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Make sure an access token can't be used as a refresh token and vice versa
	if claims.Type != expectedType {
		return nil, fmt.Errorf("invalid token type: expected %s, got %s", expectedType, claims.Type)
	}

	return claims, nil
}
