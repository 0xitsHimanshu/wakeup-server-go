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

type PingRequest struct {
	URL     string `json:"url" binding:"required,url"`
	WebHook string `json:"webHook"`
}

func CreatePingHandler(c *gin.Context) {
	var pingReq PingRequest

	email, emailExists := c.Get("email")
	if !emailExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid token claims",
			"details": "Email not found in token",
		})
		return // Missing return was causing the function to continue
	}

	if err := c.ShouldBindJSON(&pingReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	var user models.User
	// Use First instead of Find - returns error if not found
	if err := database.DB.Preload("Tasks").Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "User not found",
			"details": err.Error(),
		})
		return
	}

	// Check task limit
	if len(user.Tasks) >= 5 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Task limit reached",
			"details": "You can only have 5 active tasks at a time",
		})
		return
	}

	// Check for duplicate URL
	for _, task := range user.Tasks {
		if task.URL == pingReq.URL {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Task already exists",
				"details": "A task with this URL already exists",
			})
			return
		}
	}

	// Build new task - handle optional webhook safely
	newTask := models.Task{
		URL:           pingReq.URL,
		IsActive:      true,
		NotifyDiscord: pingReq.WebHook != "",
		WebHook:       pingReq.WebHook, // Empty string if not provided
		UserID:        user.ID,
		FailCount:     0,
	}

	if err := database.DB.Create(&newTask).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create task",
			"details": err.Error(),
		})
		return
	}

	// Schedule first ping in the queue (10 minutes from now)
	redisClient := libraries.GetInstance()
	taskMember := fmt.Sprintf("%d|%s", newTask.ID, newTask.URL)
	nextPing := time.Now().Add(worker.PingInterval).Unix()

	if _, err := redisClient.ZAdd(c.Request.Context(), "ping_queue", &redis.Z{
		Score:  float64(nextPing),
		Member: taskMember,
	}).Result(); err != nil {
		// Log error but don't fail the request - task is created
		fmt.Printf("Warning: Failed to add task %d to ping queue: %v\n", newTask.ID, err)
	}

	// Perform immediate ping in background with timeout context
	go func(taskID uint, url string) {
		ctx, cancel := context.WithTimeout(context.Background(), worker.PingTimeout)
		defer cancel()
		worker.PerformPing(ctx, redisClient, taskID, url)
	}(newTask.ID, newTask.URL)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Task created successfully and initial ping started",
		"url":     pingReq.URL,
		"taskId":  newTask.ID,
	})
}
