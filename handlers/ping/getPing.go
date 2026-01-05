package ping

import (
	"net/http"
	"wakeup-server-go/database"
	"wakeup-server-go/models"

	"github.com/gin-gonic/gin"
)

func GetPingsHandler(c *gin.Context) {
	email, emailExists := c.Get("email")
	if !emailExists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid token claims",
			"details": "Email not found in token",
		})
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "User not found",
			"details": err.Error(),
		})
		return
	}

	// Fetch tasks with logs (already limited to 10 per task by worker)
	var tasks []models.Task
	if err := database.DB.
		Where("user_id = ?", user.ID).
		Preload("Logs").
		Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch tasks",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pings fetched successfully",
		"pings":   tasks,
		"count":   len(tasks),
	})
}
