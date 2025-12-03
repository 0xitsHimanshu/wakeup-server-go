package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"wakeup-server-go/database"
	"wakeup-server-go/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type UserInfo struct {
	Email string `json:"email"`
}

type CustomClaims struct {
	UserId uint   `json:"userId"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GoogleAuth(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authorization header missing"})
		c.Abort()
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Authorization header format"})
		c.Abort()
		return
	}

	accessToken := parts[1]

	resp, err := http.Get("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=" + accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Invalid Google access token"})
		c.Abort()
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid Google access token"})
		c.Abort()
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to read Google response"})
		c.Abort()
		return
	}
	var userInfo UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to parse Google response"})
		c.Abort()
		return
	}

	var user models.User
	result := database.DB.Where("email = ?", userInfo.Email).First(&user)
	if result.Error != nil {
		// User not found, create new user
		user = models.User{Email: userInfo.Email}
		if err := database.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create user", "error": err.Error()})
			c.Abort()
			return
		}
	}

	signedToken, err := GetSignedToken(user.ID, userInfo.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to Sign token"})
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Authentication successful",
		"token":   signedToken,
	})
}

func GetSignedToken(userId uint, email string) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	claims := CustomClaims{
		UserId: userId,
		Email:  email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}
