package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateBody is a middleware that validates the request body against the provided struct

func ValidateBody(objFactory func() interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {

		obj := objFactory()
		
		if err := c.BindJSON(obj); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid request body: " + err.Error(),
			})
			c.Abort()
			return
		}

		if err := validate.Struct(obj); err != nil {
			errors := make(map[string]string)
			for _, err := range err.(validator.ValidationErrors) {
				errors[err.Field()] = err.Tag()
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"validationErrors":  errors,
			})
			c.Abort()
			return
		}

		c.Set("validatedBody", obj)
		c.Next()
	}
}