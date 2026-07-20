package main

import (
	"context"
	"log"

	"github.com/bard/bard-backend/internal/config"
	"github.com/bard/bard-backend/internal/db"
	"github.com/bard/bard-backend/internal/email"
	"github.com/bard/bard-backend/internal/handler"
	"github.com/bard/bard-backend/internal/repository"
	"github.com/bard/bard-backend/internal/router"
	"github.com/bard/bard-backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	// Database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to database")

	// Repositories
	userRepo := repository.NewUserRepository(pool)

	// Services
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.JWTRefreshSecret)
	googleOAuth := service.NewGoogleOAuthService(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.AppBaseURL+"/auth/google/callback")
	magicLinkSvc := service.NewMagicLinkService(pool)
	mailer := email.NewMailer(cfg.ResendAPIKey, cfg.ResendFromAddr, cfg.AppBaseURL)

	// Handlers
	healthH := handler.NewHealthHandler(pool)
	authH := handler.NewAuthHandler(tokenService, googleOAuth, magicLinkSvc, userRepo, mailer)

	// Router
	r := router.Setup(healthH, authH, tokenService)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
