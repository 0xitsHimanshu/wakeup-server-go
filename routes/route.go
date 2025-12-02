package routes

import (
	"net/http"
	"wakeup-server-go/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	apiGroup := r.Group("/api/v1")
	
	// Public routes (no authentication required)
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Server is healthy"})
	})
	apiGroup.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "test response"})
	})

	// Protected routes (require JWT authentication)
	protectedGroup := apiGroup.Group("/protected")
	protectedGroup.Use(middleware.TokenValidation)
	{
		// Mock route to test the middleware
		protectedGroup.GET("/profile", func(c *gin.Context) {
			// Extract user information from context (set by middleware)
			email, emailExists := c.Get("email")
			userId, userIdExists := c.Get("userId")

			if !emailExists || !userIdExists {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "User information not found in context",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Access granted to protected resource",
				"data": gin.H{
					"email":  email,
					"userId": userId,
				},
			})
		})

		// Another protected route example
		protectedGroup.GET("/dashboard", func(c *gin.Context) {
			email, _ := c.Get("email")
			
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Welcome to your dashboard",
				"user":    email,
			})
		})
	}
}