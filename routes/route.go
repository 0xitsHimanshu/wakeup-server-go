package routes

import (
	"net/http"
	"wakeup-server-go/handlers/auth"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	apiGroup := r.Group("/api/v1")
	apiGroup.GET("/auth/google", auth.GoogleAuth) // Public route for Google authentication
	apiGroup.GET("/health", func(c *gin.Context) { // Public routes (no authentication required)
		c.JSON(http.StatusOK, gin.H{"message": "Server is healthy"})
	})	
}
