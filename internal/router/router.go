package router

import (
	"github.com/bard/bard-backend/internal/handler"
	"github.com/gin-gonic/gin"
)

func Setup(healthH *handler.HealthHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/health", healthH.Health)

	return r
}
