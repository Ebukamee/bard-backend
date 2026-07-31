package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	JWTSecret          string
	JWTRefreshSecret   string
	GoogleClientID     string
	GoogleClientSecret string
	CloudinaryURL      string
	AppBaseURL         string
	ResendAPIKey       string
	ResendFromAddr     string
	GeminiAPIKey       string
	GroqAPIKey         string
	AllowedOrigins     []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		JWTRefreshSecret:   os.Getenv("JWT_REFRESH_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		CloudinaryURL:      os.Getenv("CLOUDINARY_URL"),
		AppBaseURL:         getEnv("APP_BASE_URL", "http://localhost:3000"),
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		ResendFromAddr:     getEnv("RESEND_FROM", "Bard <noreply@yourdomain.com>"),
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		GroqAPIKey:         os.Getenv("GROQ_API_KEY"),
		AllowedOrigins:     parseOrigins(getEnv("ALLOWED_ORIGINS", "http://localhost:3000")),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.JWTRefreshSecret == "" {
		return nil, fmt.Errorf("JWT_REFRESH_SECRET is required")
	}

	return cfg, nil
}

func parseOrigins(s string) []string {
	var origins []string
	for _, o := range strings.Split(s, ",") {
		if t := strings.TrimSpace(o); t != "" {
			origins = append(origins, t)
		}
	}
	return origins
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
