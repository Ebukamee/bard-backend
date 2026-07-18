package middleware

import (
	"net/http"
	"strings"

	"github.com/bard/bard-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthRequired returns a middleware that blocks unauthenticated requests
func AuthRequired(tokenService *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get the Authorization header
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			return
		}

		// 2. Check it starts with "Bearer "
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization format, expected: Bearer <token>",
			})
			return
		}

		// 3. Extract the token string (everything after "Bearer ")
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		// 4. Validate the token and extract claims
		claims, err := tokenService.ValidateAccessToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		// 5. Store user info in the request context for handlers to use
		c.Set("user_id", claims.Subject)
		c.Set("user_email", claims.Email)

		// 6. Continue to the next handler
		c.Next()
	}
}
