package router

import (
	"time"

	"github.com/bard/bard-backend/internal/handler"
	"github.com/bard/bard-backend/internal/middleware"
	"github.com/bard/bard-backend/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Setup(
	healthH *handler.HealthHandler,
	authH *handler.AuthHandler,
	transcriptionH *handler.TranscriptionHandler,
	tokenService *service.TokenService,
) *gin.Engine {
	r := gin.Default()

	// CORS — allow frontend origins
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", healthH.Health)

	// Public auth routes (no token needed)
	auth := r.Group("/auth")
	{
		auth.POST("/google", authH.GoogleAuth)
		auth.POST("/magic-link/request", authH.RequestMagicLink)
		auth.POST("/magic-link/verify", authH.VerifyMagicLink)
		auth.POST("/refresh", authH.RefreshToken)
	}

	// Protected routes (require valid access token)
	protected := r.Group("/")
	protected.Use(middleware.AuthRequired(tokenService))
	{
		protected.GET("/auth/me", authH.Me)
		protected.PATCH("/profile", authH.UpdateProfile)

		// Transcription routes
		protected.POST("/transcriptions/upload", transcriptionH.Upload)
		protected.GET("/transcriptions", transcriptionH.List)
		protected.GET("/transcriptions/:id", transcriptionH.Get)
		protected.GET("/transcriptions/:id/audio", transcriptionH.Audio)
		protected.DELETE("/transcriptions/:id", transcriptionH.Delete)
	}

	return r
}
