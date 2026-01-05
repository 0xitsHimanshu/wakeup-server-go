package routes

import (
	"wakeup-server-go/handlers/ping"
	"wakeup-server-go/middleware"

	"github.com/gin-gonic/gin"
)

type PingRequest struct {
	Url string `json:"url" validate:"required,url"`	
}

func pingRouter(r *gin.RouterGroup) {
	r.POST("/create", middleware.TokenValidation, ping.CreatePingHandler)
	r.DELETE("/delete", middleware.TokenValidation, ping.DeletePingHandler)
	r.GET("/getall", middleware.TokenValidation, ping.GetPingsHandler)
	r.PATCH("/reactivate", middleware.TokenValidation, ping.ReactivatePingHandler)
}