package worker

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"wakeup-server-go/database"
	"wakeup-server-go/libraries"
	"wakeup-server-go/models"

	"github.com/go-redis/redis/v8"
)

const (
	MaxLogsPerTask     = 10
	MaxConcurrentPings = 10               // Limit concurrent pings to avoid overwhelming resources
	PingTimeout        = 30 * time.Second // Timeout for individual ping requests
	PingInterval       = 10 * time.Minute
	WorkerPollInterval = 30 * time.Second // Check queue more frequently for responsiveness
)

// Reusable HTTP client with connection pooling and timeouts
var httpClient = &http.Client{
	Timeout: PingTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	},
}

// PerformPing executes an HTTP ping to the given URL and logs the result
func PerformPing(ctx context.Context, redisClient *redis.Client, taskID uint, url string) {
	taskMember := fmt.Sprintf("%d|%s", taskID, url)

	// Trim logs asynchronously - don't block the ping
	go func() {
		if err := TrimLogs(taskID); err != nil {
			log.Printf("Error trimming logs for task %d: %v", taskID, err)
		}
	}()

	timeNow := time.Now()

	// Create request with context for proper cancellation
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logPingFailure(taskID, 0, time.Since(timeNow).Milliseconds(), "Failed to create request")
		log.Printf("Failed to create request for %s: %v", url, err)
		return
	}

	resp, err := httpClient.Do(req)
	timeSince := time.Since(timeNow).Milliseconds()

	if err != nil {
		logPingFailure(taskID, 0, timeSince, "Failed to ping URL")
		handlePingFailure(ctx, redisClient, taskID, url, taskMember)
		log.Printf("Failed to ping %s: %v", url, err)
		return
	}

	// Drain and close body to enable connection reuse
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// Consider 2xx and 3xx status codes as success (Server is responding)
	isSuccess := resp.StatusCode >= 200 && resp.StatusCode < 400

	if !isSuccess {
		logPingFailure(taskID, resp.StatusCode, timeSince, fmt.Sprintf("Server responded with status: %d", resp.StatusCode))
		handlePingFailure(ctx, redisClient, taskID, url, taskMember)
		log.Printf("Ping failed for %s with status code: %d", url, resp.StatusCode)
		return
	}

	// Success: Schedule next ping and reset fail count
	nextPing := time.Now().Add(PingInterval).Unix()
	if _, err := redisClient.ZAdd(ctx, "ping_queue", &redis.Z{
		Score:  float64(nextPing),
		Member: taskMember,
	}).Result(); err != nil {
		log.Printf("Error scheduling next ping for %s: %v", url, err)
	}

	// Reset fail count on success
	database.DB.Model(&models.Task{}).Where("id = ?", taskID).Update("fail_count", 0)

	// Log success
	newLog := models.Log{
		LogResponse:  fmt.Sprintf("Server is up (Status: %d)", resp.StatusCode),
		Time:         time.Now(),
		TimeTake:     timeSince,
		TaskID:       taskID,
		IsSuccess:    true,
		ResponseCode: resp.StatusCode,
	}
	if err := database.DB.Create(&newLog).Error; err != nil {
		log.Printf("Error creating log: %v", err)
	}
	log.Printf("Successfully pinged %s - Status %d (took %dms)", url, resp.StatusCode, timeSince)
}

// logPingFailure creates a failure log entry
func logPingFailure(taskID uint, statusCode int, timeTaken int64, message string) {
	newLog := models.Log{
		LogResponse:  message,
		Time:         time.Now(),
		TimeTake:     timeTaken,
		TaskID:       taskID,
		IsSuccess:    false,
		ResponseCode: statusCode,
	}
	if err := database.DB.Create(&newLog).Error; err != nil {
		log.Printf("Error creating failure log: %v", err)
	}
}

// handlePingFailure updates task fail count and handles deactivation
func handlePingFailure(ctx context.Context, redisClient *redis.Client, taskID uint, url, taskMember string) {
	var task models.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		log.Printf("Error fetching task %d: %v", taskID, err)
		return
	}

	task.FailCount++

	if task.FailCount >= 2 {
		// Deactivate task after 2 consecutive failures
		task.IsActive = false
		if err := database.DB.Save(&task).Error; err != nil {
			log.Printf("Error deactivating task %d: %v", taskID, err)
		}
		redisClient.ZRem(ctx, "ping_queue", taskMember)
		redisClient.LPush(ctx, "noti_queue", taskID)
		log.Printf("Task %d deactivated after %d failures", taskID, task.FailCount)
	} else {
		// Update fail count and reschedule with shorter interval
		if err := database.DB.Model(&task).Update("fail_count", task.FailCount).Error; err != nil {
			log.Printf("Error updating fail count for task %d: %v", taskID, err)
		}
		nextPing := time.Now().Add(PingInterval).Unix()
		if _, err := redisClient.ZAdd(ctx, "ping_queue", &redis.Z{
			Score:  float64(nextPing),
			Member: taskMember,
		}).Result(); err != nil {
			log.Printf("Error rescheduling URL %s: %v", url, err)
		}
	}
}

// StartPingWorker starts the main ping worker loop with graceful shutdown support
func StartPingWorker(ctx context.Context) {
	redisClient := libraries.GetInstance()
	semaphore := make(chan struct{}, MaxConcurrentPings)
	var wg sync.WaitGroup

	log.Println("Ping worker started")

	ticker := time.NewTicker(WorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Ping worker shutting down, waiting for in-flight pings...")
			wg.Wait()
			log.Println("Ping worker stopped")
			return
		case <-ticker.C:
			processPendingTasks(ctx, redisClient, semaphore, &wg)
		}
	}
}

// processPendingTasks fetches and processes all due tasks
func processPendingTasks(ctx context.Context, redisClient *redis.Client, semaphore chan struct{}, wg *sync.WaitGroup) {
	now := time.Now().Unix()

	tasks, err := redisClient.ZRangeByScore(ctx, "ping_queue", &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()
	if err != nil {
		log.Printf("Error fetching from queue: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	log.Printf("Processing %d pending tasks", len(tasks))

	for _, task := range tasks {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		parts := strings.SplitN(task, "|", 2)
		if len(parts) != 2 {
			log.Printf("Invalid task format: %s", task)
			redisClient.ZRem(ctx, "ping_queue", task)
			continue
		}

		taskIDStr, url := parts[0], parts[1]
		taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
		if err != nil {
			log.Printf("Invalid task ID: %s", taskIDStr)
			redisClient.ZRem(ctx, "ping_queue", task)
			continue
		}

		// Remove from queue before processing to avoid duplicates
		redisClient.ZRem(ctx, "ping_queue", task)

		// Acquire semaphore slot for concurrency control
		semaphore <- struct{}{}
		wg.Add(1)

		go func(id uint, pingURL string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Create a timeout context for individual pings
			pingCtx, cancel := context.WithTimeout(ctx, PingTimeout)
			defer cancel()

			PerformPing(pingCtx, redisClient, id, pingURL)
		}(uint(taskID), url)
	}
}

// TrimLogs removes old logs to keep only the most recent MaxLogsPerTask entries
func TrimLogs(taskID uint) error {
	var logCount int64
	if err := database.DB.Model(&models.Log{}).Where("task_id = ?", taskID).Count(&logCount).Error; err != nil {
		return fmt.Errorf("failed to count logs: %w", err)
	}

	if logCount > MaxLogsPerTask {
		// Delete excess logs in a single query using subquery
		excessCount := int(logCount - MaxLogsPerTask)
		result := database.DB.Exec(`
			DELETE FROM logs 
			WHERE id IN (
				SELECT id FROM logs 
				WHERE task_id = ? 
				ORDER BY time ASC 
				LIMIT ?
			)`, taskID, excessCount)

		if result.Error != nil {
			return fmt.Errorf("failed to trim logs: %w", result.Error)
		}
	}
	return nil
}

// Legacy function for backward compatibility - wraps PerformPing with background context
func PeformPing(taskID uint, url string) {
	log.Printf("Performing immediate ping for task %d: %s", taskID, url)
	redisClient := libraries.GetInstance()
	PerformPing(context.Background(), redisClient, taskID, url)
}
