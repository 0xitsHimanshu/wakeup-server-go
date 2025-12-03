package utils

import (
	"log"
	"net/http"
	"github.com/gin-gonic/gin"
)

type HandleFucnWithError func(c *gin.Context) error

// ErrorHandler is a middleware that handles errors returned by handlers
func ErrorHandler(handler HandleFucnWithError) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := handler(c); err != nil {
			log.Printf("Error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{ "status":  "error", "message": err.Error()})
			c.Abort() // Stop further handlers
		}
	}
}