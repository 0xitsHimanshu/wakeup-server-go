package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Email        string `json:"email" binding:"required,email" gorm:"unique;not null"`
	Password     string `json:"-" gorm:"not null"`
	RefreshToken string `json:"-"`
	Tasks        []Task `json:"tasks" gorm:"foreignKey:UserID"`
}

type Task struct {
	gorm.Model
	URL           string `json:"url" binding:"required,url"`
	IsActive      bool   `json:"isActive"`
	NotifyDiscord bool   `json:"notifyDiscord" gorm:"default:false"`
	WebHook       string `json:"webHook" gorm:"default:null"`
	UserID        uint   `json:"userId" binding:"required"`
	Logs          []Log  `json:"logs" gorm:"foreignKey:TaskID"`
	FailCount     int    `json:"failCount" gorm:"default:0"`
}

type Log struct {
	gorm.Model
	TaskID       uint      `json:"taskId" binding:"required" `
	Timestamp    time.Time `json:"time"`
	TimeTake     int64     `json:"timeTake"`
	LogResponse  string    `json:"logResponse"`
	IsSuccess    bool      `json:"isSuccess"`
	ResponseCode int       `json:"responseCode"`
}
