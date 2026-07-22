package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)


type GoogleUserInfo struct {
	ID        string `json:"id"`         
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"picture"`    
	Verified  bool   `json:"verified_email"`
}


type GoogleOAuthService struct {
	config *oauth2.Config
}

// NewGoogleOAuthService creates the service with your Google app credentials
func NewGoogleOAuthService(clientID, clientSecret, redirectURL string) *GoogleOAuthService {
	return &GoogleOAuthService{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// ExchangeCode takes the authorization code from Google and returns the user's profile info.
// This is the core of the OAuth flow:
//   1. User clicks "Sign in with Google" → Google shows consent screen
//   2. User approves → Google redirects back to your app with a "code"
//   3. Your frontend sends that code to your backend
//   4. This function exchanges the code for an access token, then fetches the profile
func (s *GoogleOAuthService) ExchangeCode(ctx context.Context, code string) (*GoogleUserInfo, error) {
	// Step 1: Exchange the authorization code for an access token
	// This is a server-to-server call (your backend → Google's servers)
	// The code can only be used ONCE and expires in ~10 minutes
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Step 2: Use the access token to fetch the user's profile
	userInfo, err := s.fetchUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	// Step 3: Make sure the email is verified (security check)
	if !userInfo.Verified {
		return nil, fmt.Errorf("google email is not verified")
	}

	return userInfo, nil
}

// SetRedirectURL allows overriding the redirect URL per request.
// Useful when your app has multiple frontends (web, mobile) with different callback URLs.
func (s *GoogleOAuthService) SetRedirectURL(url string) {
	s.config.RedirectURL = url
}

// fetchUserInfo calls Google's userinfo API with the access token
func (s *GoogleOAuthService) fetchUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	// Create an HTTP client that automatically attaches the access token to requests
	client := s.config.Client(ctx, token)

	// Call Google's userinfo endpoint
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to call google userinfo API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response into our struct
	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode google user info: %w", err)
	}

	return &userInfo, nil
}
