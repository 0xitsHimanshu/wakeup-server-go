package main

import (
	"log"
	"os"
	"time"

	"wakeup-server-go/database"
	"wakeup-server-go/models"
	"wakeup-server-go/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main(){
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	database.Connect()
	if err := database.AutoMigrate(&models.User{}, &models.Log{}, &models.Task{}); err != nil {
		log.Fatal("Error during database migration:", err)
	}

	if err != nil {
		log.Fatal("Error conecting to database:", err)
	}

	r := gin.Default()
	
	// Configure CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // In production, replace with your frontend URL
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	
	routes.SetupRouter(r)

	PORT := os.Getenv("PORT")


	r.Run(":" + PORT);
}