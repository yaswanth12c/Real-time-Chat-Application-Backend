package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yaswa/go-chat-backend/internal/auth"
	"github.com/yaswa/go-chat-backend/internal/models"
)

// CreateRoom creates a new chat room
func CreateRoom(c *gin.Context) {
	var input models.CreateRoomInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserIDFromContext(c)
	room, err := models.CreateRoom(&input, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create room"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Room created",
		"room":    room,
	})
}

// ListRooms returns rooms the authenticated user is a member of
func ListRooms(c *gin.Context) {
	userID := auth.GetUserIDFromContext(c)
	rooms, err := models.ListUserRooms(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}

	if rooms == nil {
		rooms = []models.ChatRoom{}
	}

	c.JSON(http.StatusOK, gin.H{"rooms": rooms})
}

// GetRoom returns details of a specific room
func GetRoom(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	room, err := models.GetRoomByID(roomID)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	// Check membership for private rooms
	if room.IsPrivate {
		userID := auth.GetUserIDFromContext(c)
		isMember, _ := models.IsMember(roomID, userID)
		if !isMember {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to private room"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"room": room})
}

// UpdateRoom updates a chat room (owner only)
func UpdateRoom(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := auth.GetUserIDFromContext(c)
	isOwner, _ := models.IsRoomOwner(roomID, userID)
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the room owner can update the room"})
		return
	}

	var input models.UpdateRoomInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room, err := models.UpdateRoom(roomID, &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Room updated",
		"room":    room,
	})
}

// DeleteRoom deletes a chat room (owner only)
func DeleteRoom(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := auth.GetUserIDFromContext(c)
	isOwner, _ := models.IsRoomOwner(roomID, userID)
	if !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the room owner can delete the room"})
		return
	}

	if err := models.DeleteRoom(roomID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Room deleted"})
}

// JoinRoom adds the authenticated user to a room
func JoinRoom(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	room, err := models.GetRoomByID(roomID)
	if err != nil || room == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	if room.IsPrivate {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot join a private room without an invite"})
		return
	}

	userID := auth.GetUserIDFromContext(c)
	already, _ := models.IsMember(roomID, userID)
	if already {
		c.JSON(http.StatusConflict, gin.H{"error": "Already a member of this room"})
		return
	}

	if err := models.AddMember(roomID, userID, "member"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Joined room successfully"})
}

// LeaveRoom removes the authenticated user from a room
func LeaveRoom(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := auth.GetUserIDFromContext(c)

	isOwner, _ := models.IsRoomOwner(roomID, userID)
	if isOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Room owner cannot leave. Delete the room or transfer ownership."})
		return
	}

	isMember, _ := models.IsMember(roomID, userID)
	if !isMember {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not a member of this room"})
		return
	}

	if err := models.RemoveMember(roomID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Left room successfully"})
}

// GetRoomMembers lists all members of a room
func GetRoomMembers(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID := auth.GetUserIDFromContext(c)
	isMember, _ := models.IsMember(roomID, userID)
	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "Must be a room member to view members"})
		return
	}

	members, err := models.GetRoomMembers(roomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
		return
	}

	if members == nil {
		members = []models.ChatRoomMember{}
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}
