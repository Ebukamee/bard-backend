package router

import (
	"github.com/bard/bard-backend/internal/handler"
	"github.com/bard/bard-backend/internal/middleware"
	"github.com/bard/bard-backend/internal/service"
	"github.com/gin-gonic/gin"
)

func Setup(
	healthH *handler.HealthHandler,
	authH *handler.AuthHandler,
	tokenService *service.TokenService,
) *gin.Engine {
	r := gin.Default()

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
	}

	return r
}
