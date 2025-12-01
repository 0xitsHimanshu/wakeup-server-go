package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	apiGroup := r.Group("/api/v1")
	apiGroup.GET("/health", func(c *gin.Context)  {
		c.JSON(http.StatusOK, gin.H{"message": "Server is healthy"})
	})
	apiGroup.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "test response"})
	})
}