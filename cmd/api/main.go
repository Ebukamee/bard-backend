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
	"github.com/bard/bard-backend/internal/worker"
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

	// Run migrations
	if err := db.AutoMigrate(ctx, pool); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	transcriptionRepo := repository.NewTranscriptionRepository(pool)

	// Services
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.JWTRefreshSecret)
	googleOAuth := service.NewGoogleOAuthService(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.AppBaseURL+"/auth/google/callback")
	magicLinkSvc := service.NewMagicLinkService(pool)
	mailer := email.NewMailer(cfg.ResendAPIKey, cfg.ResendFromAddr, cfg.AppBaseURL)

	storageSvc, err := service.NewStorageService(cfg.CloudinaryURL)
	if err != nil {
		log.Fatalf("Failed to create storage service: %v", err)
	}

	geminiSvc := service.NewGeminiService(cfg.GeminiAPIKey)
	whisperSvc := service.NewWhisperService(cfg.GroqAPIKey)

	// Background worker
	processor := worker.NewProcessor(transcriptionRepo, whisperSvc, geminiSvc)
	go processor.Start(ctx)

	// Handlers
	healthH := handler.NewHealthHandler(pool)
	authH := handler.NewAuthHandler(tokenService, googleOAuth, magicLinkSvc, userRepo, mailer)
	transcriptionH := handler.NewTranscriptionHandler(transcriptionRepo, storageSvc, processor)

	// Router
	r := router.Setup(cfg, healthH, authH, transcriptionH, tokenService)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
