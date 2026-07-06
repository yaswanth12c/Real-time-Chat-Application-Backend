package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yaswa/go-chat-backend/internal/auth"
	"github.com/yaswa/go-chat-backend/internal/config"
	"github.com/yaswa/go-chat-backend/internal/database"
	"github.com/yaswa/go-chat-backend/internal/models"
)

// Register creates a new user account
func Register(c *gin.Context) {
	var input models.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username already exists
	existing, err := models.GetUserByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
		return
	}

	// Check if email already exists
	existing, err = models.GetUserByEmail(input.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	user, err := models.CreateUser(&input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Login authenticates a user and returns JWT tokens
func Login(c *gin.Context) {
	var input models.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := models.GetUserByUsername(input.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !models.CheckPassword(user, input.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate session ID and store in Redis
	sessionID := uuid.New().String()
	err = database.StoreSession(user.ID, sessionID, config.AppConfig.RefreshExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	tokens, err := auth.GenerateTokenPair(user.ID, user.Username, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"tokens":  tokens,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// RefreshToken issues new tokens using a refresh token
func RefreshToken(c *gin.Context) {
	var input struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := auth.ValidateRefreshToken(input.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Check session is still valid
	_, err = database.GetSession(claims.SessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
		return
	}

	// Generate new token pair with same session
	tokens, err := auth.GenerateTokenPair(claims.UserID, claims.Username, claims.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed",
		"tokens":  tokens,
	})
}

// Logout invalidates the current session
func Logout(c *gin.Context) {
	sessionID, exists := c.Get("sessionID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active session"})
		return
	}

	err := database.DeleteSession(sessionID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Set user offline
	userID := auth.GetUserIDFromContext(c)
	models.SetUserOnlineStatus(userID, false)
	database.SetUserOffline(userID)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetProfile returns the authenticated user's profile
func GetProfile(c *gin.Context) {
	userID := auth.GetUserIDFromContext(c)
	user, err := models.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"avatar_url": user.AvatarURL,
			"is_online":  user.IsOnline,
			"last_seen":  user.LastSeen,
			"created_at": user.CreatedAt,
		},
	})
}

// UpdateProfile updates the authenticated user's profile
func UpdateProfile(c *gin.Context) {
	userID := auth.GetUserIDFromContext(c)
	var input models.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := models.UpdateUserProfile(userID, &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated",
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"avatar_url": user.AvatarURL,
		},
	})
}

// GetUserByID returns a user's public profile by ID
func GetUserByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := models.GetUserByID(id)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"avatar_url": user.AvatarURL,
			"is_online":  user.IsOnline,
			"last_seen":  user.LastSeen,
		},
	})
}
