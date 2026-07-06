package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yaswa/go-chat-backend/internal/database"
)

// AuthMiddleware validates JWT tokens and injects user info into context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format. Use: Bearer <token>"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Check if access token (not refresh)
		if claims.Subject != "access" && claims.Subject != "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token type"})
			c.Abort()
			return
		}

		// Validate session in Redis
		_, err = database.GetSession(claims.SessionID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired or revoked"})
			c.Abort()
			return
		}

		// Set user info in context for downstream handlers
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("sessionID", claims.SessionID)

		c.Next()
	}
}

// GetUserIDFromContext retrieves the authenticated user's ID from gin context
func GetUserIDFromContext(c *gin.Context) int64 {
	userID, exists := c.Get("userID")
	if !exists {
		return 0
	}
	return userID.(int64)
}

// GetUsernameFromContext retrieves the authenticated user's username from gin context
func GetUsernameFromContext(c *gin.Context) string {
	username, exists := c.Get("username")
	if !exists {
		return ""
	}
	return username.(string)
}
