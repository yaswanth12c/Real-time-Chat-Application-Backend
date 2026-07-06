package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yaswa/go-chat-backend/internal/auth"
	"github.com/yaswa/go-chat-backend/internal/models"
)

// GetMessages returns paginated message history for a room
func GetMessages(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Verify user is member of room
	userID := auth.GetUserIDFromContext(c)
	isMember, _ := models.IsMember(roomID, userID)
	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "Must be a room member to view messages"})
		return
	}

	// Parse pagination params
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := models.GetMessagesByRoom(roomID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	if messages == nil {
		messages = []models.Message{}
	}

	total, _ := models.GetRoomMessageCount(roomID)

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}
