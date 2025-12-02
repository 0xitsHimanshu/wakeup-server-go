package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	secretKey     []byte
	secretKeyOnce sync.Once
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer"
	emailClaimKey       = "email"
	userIDClaimKey      = "userId"
)

// initSecretKey initializes the JWT secret key from environment variable
func initSecretKey() {
	secretKeyOnce.Do(func() {
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			panic("JWT_SECRET environment variable is not set")
		}
		secretKey = []byte(jwtSecret)
	})
}

// extractBearerToken extracts the token from the Authorization header
func extractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != bearerPrefix {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

// parseAndValidateToken parses the JWT token and validates it
func parseAndValidateToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	return token, nil
}

// extractClaims extracts and validates claims from the token
func extractClaims(token *jwt.Token) (jwt.MapClaims, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// TokenValidation is a middleware that validates JWT tokens
func TokenValidation(c *gin.Context) {
	initSecretKey()

	authHeader := c.GetHeader(authorizationHeader)
	tokenString, err := extractBearerToken(authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		c.Abort()
		return
	}

	token, err := parseAndValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired token"})
		c.Abort()
		return
	}

	claims, err := extractClaims(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		c.Abort()
		return
	}

	// Set claims in context for use in handlers
	if email, ok := claims[emailClaimKey]; ok {
		c.Set(emailClaimKey, email)
	}
	if userID, ok := claims[userIDClaimKey]; ok {
		c.Set(userIDClaimKey, userID)
	}

	c.Next()
}
