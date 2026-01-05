package ping

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"wakeup-server-go/database"
	"wakeup-server-go/libraries"
	"wakeup-server-go/models"
	"wakeup-server-go/worker"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type ReactivatePingRequest struct {
	TaskID uint `json:"taskId" binding:"required"`
}

func ReactivatePingHandler(c *gin.Context) {
	var pingReq ReactivatePingRequest

	email, emailExists := c.Get("email")
	if !emailExists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid token claims",
			"details": "Email not found in token",
		})
		return
	}

	if err := c.ShouldBindJSON(&pingReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Find user - no need to preload tasks
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "User not found",
			"details": err.Error(),
		})
		return
	}

	// Find task directly
	var task models.Task
	if err := database.DB.Where("id = ? AND user_id = ?", pingReq.TaskID, user.ID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Task not found for this user",
			"details": err.Error(),
		})
		return
	}

	// Check if already active
	if task.IsActive {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Task already active",
			"details": "This task is already running",
		})
		return
	}

	// Reactivate and reset fail count
	if err := database.DB.Model(&task).Updates(map[string]interface{}{
		"is_active":  true,
		"fail_count": 0,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to reactivate task",
			"details": err.Error(),
		})
		return
	}

	// Add to ping queue for immediate ping
	redisClient := libraries.GetInstance()
	taskMember := fmt.Sprintf("%d|%s", task.ID, task.URL)
	nextPing := time.Now().Add(10 * time.Second).Unix() // Quick first ping

	if _, err := redisClient.ZAdd(c.Request.Context(), "ping_queue", &redis.Z{
		Score:  float64(nextPing),
		Member: taskMember,
	}).Result(); err != nil {
		// Log but don't fail - task is reactivated, worker will pick it up
		fmt.Printf("Warning: Failed to add task %d to ping queue: %v\n", task.ID, err)
	}

	// Perform immediate ping in background
	go func(taskID uint, url string) {
		ctx, cancel := context.WithTimeout(context.Background(), worker.PingTimeout)
		defer cancel()
		worker.PerformPing(ctx, redisClient, taskID, url)
	}(task.ID, task.URL)

	c.JSON(http.StatusOK, gin.H{
		"message": "Task reactivated successfully",
		"taskId":  task.ID,
		"url":     task.URL,
	})
}
