package ping

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"wakeup-server-go/database"
	"wakeup-server-go/libraries"
	"wakeup-server-go/models"

	"github.com/gin-gonic/gin"
)

type DeletePingRequest struct {
	TaskID uint `json:"taskId" binding:"required"`
}

func DeletePingHandler(c *gin.Context) {
	var delPingReq DeletePingRequest

	email, emailExists := c.Get("email")
	if !emailExists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid token claims",
			"details": "Email not found in token",
		})
		return // Missing return was causing the function to continue
	}

	if err := c.ShouldBindJSON(&delPingReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Find user first
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "User not found",
			"details": err.Error(),
		})
		return
	}

	// Find the task directly - more efficient than loading all tasks
	var task models.Task
	if err := database.DB.Where("id = ? AND user_id = ?", delPingReq.TaskID, user.ID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Task not found",
			"details": "Task does not exist or doesn't belong to you",
		})
		return
	}

	// Use transaction to ensure atomic deletion
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete logs FIRST (before task) to maintain referential integrity
	if err := tx.Where("task_id = ?", task.ID).Delete(&models.Log{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete logs",
			"details": err.Error(),
		})
		return
	}

	// Delete the task
	if err := tx.Delete(&task).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete task",
			"details": err.Error(),
		})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to complete deletion",
			"details": err.Error(),
		})
		return
	}

	// Remove from Redis queue (fire and forget - don't fail request if this fails)
	go func(taskID uint, url string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		redisClient := libraries.GetInstance()
		taskMember := fmt.Sprintf("%d|%s", taskID, url)
		if err := redisClient.ZRem(ctx, "ping_queue", taskMember).Err(); err != nil {
			fmt.Printf("Warning: Failed to remove task %d from ping queue: %v\n", taskID, err)
		}
	}(task.ID, task.URL)

	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
		"taskId":  delPingReq.TaskID,
	})
}
