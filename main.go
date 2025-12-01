package main

import (
	"log"

	"wakeup-server-go/database"
	"wakeup-server-go/models"

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

}