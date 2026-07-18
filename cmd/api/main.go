package main

import (
	"context"
	"log"

	"github.com/bard/bard-backend/internal/config"
	"github.com/bard/bard-backend/internal/db"
	"github.com/bard/bard-backend/internal/handler"
	"github.com/bard/bard-backend/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("Connected to database")

	healthH := handler.NewHealthHandler(pool)

	r := router.Setup(healthH)

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
