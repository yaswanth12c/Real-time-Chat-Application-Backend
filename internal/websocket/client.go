package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gorillaWS "github.com/gorilla/websocket"
	"github.com/yaswa/go-chat-backend/internal/auth"
	"github.com/yaswa/go-chat-backend/internal/database"
	"github.com/yaswa/go-chat-backend/internal/models"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this interval (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 4096
)

var upgrader = gorillaWS.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Client represents a single WebSocket connection
type Client struct {
	hub      *Hub
	conn     *gorillaWS.Conn
	send     chan *WSMessage
	UserID   int64
	Username string
	rooms    map[int64]bool
}

// ServeWS handles WebSocket upgrade requests
func ServeWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Authenticate via token query parameter
		tokenString := c.Query("token")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token required"})
			return
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Verify session is active
		_, err = database.GetSession(claims.SessionID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			return
		}

		// Upgrade HTTP to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}

		client := &Client{
			hub:      hub,
			conn:     conn,
			send:     make(chan *WSMessage, 256),
			UserID:   claims.UserID,
			Username: claims.Username,
			rooms:    make(map[int64]bool),
		}

		hub.register <- client

		// Update user online status in DB
		models.SetUserOnlineStatus(claims.UserID, true)

		// Auto-join user's rooms
		userRooms, err := models.ListUserRooms(claims.UserID)
		if err == nil {
			for _, room := range userRooms {
				hub.JoinRoom(room.ID, client)
			}
		}

		// Start read and write pumps in separate goroutines
		go client.writePump()
		go client.readPump()
	}
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		models.SetUserOnlineStatus(c.UserID, false)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if gorillaWS.IsUnexpectedCloseError(err, gorillaWS.CloseGoingAway, gorillaWS.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var incoming IncomingMessage
		if err := json.Unmarshal(message, &incoming); err != nil {
			c.sendError("Invalid message format")
			continue
		}

		c.handleMessage(&incoming)
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(gorillaWS.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Failed to marshal message: %v", err)
				continue
			}

			if err := c.conn.WriteMessage(gorillaWS.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gorillaWS.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage routes incoming messages by type
func (c *Client) handleMessage(msg *IncomingMessage) {
	switch msg.Type {
	case MessageTypeChat:
		c.handleChatMessage(msg)
	case MessageTypeJoinRoom:
		c.handleJoinRoom(msg)
	case MessageTypeLeaveRoom:
		c.handleLeaveRoom(msg)
	case MessageTypeTyping:
		c.handleTyping(msg)
	case MessageTypeStopTyping:
		c.handleStopTyping(msg)
	default:
		c.sendError("Unknown message type: " + msg.Type)
	}
}

// handleChatMessage processes a new chat message
func (c *Client) handleChatMessage(msg *IncomingMessage) {
	if msg.RoomID == 0 || msg.Content == "" {
		c.sendError("Room ID and content are required")
		return
	}

	// Check rate limit (30 messages per minute)
	allowed, err := database.CheckRateLimit(c.UserID, 30, time.Minute)
	if err != nil || !allowed {
		c.sendError("Rate limit exceeded. Please slow down.")
		return
	}

	// Verify membership
	isMember, _ := models.IsMember(msg.RoomID, c.UserID)
	if !isMember {
		c.sendError("Not a member of this room")
		return
	}

	// Persist message to MySQL
	savedMsg, err := models.SaveMessage(msg.RoomID, c.UserID, msg.Content, "text")
	if err != nil {
		log.Printf("Failed to save message: %v", err)
		c.sendError("Failed to send message")
		return
	}

	// Broadcast to room
	wsMsg := &WSMessage{
		Type:      MessageTypeChat,
		RoomID:    msg.RoomID,
		Content:   savedMsg.Content,
		SenderID:  c.UserID,
		Sender:    c.Username,
		Timestamp: savedMsg.CreatedAt,
		Data: map[string]interface{}{
			"message_id": savedMsg.ID,
		},
	}

	c.hub.broadcast <- &BroadcastMessage{
		RoomID:  msg.RoomID,
		Message: wsMsg,
		Sender:  c,
	}
}

// handleJoinRoom subscribes client to a room's broadcasts
func (c *Client) handleJoinRoom(msg *IncomingMessage) {
	if msg.RoomID == 0 {
		c.sendError("Room ID is required")
		return
	}

	isMember, _ := models.IsMember(msg.RoomID, c.UserID)
	if !isMember {
		c.sendError("Not a member of this room. Join via REST API first.")
		return
	}

	c.hub.JoinRoom(msg.RoomID, c)

	// Notify room
	notification := &WSMessage{
		Type:      MessageTypeSystem,
		RoomID:    msg.RoomID,
		Content:   c.Username + " is now online",
		SenderID:  c.UserID,
		Sender:    "system",
		Timestamp: time.Now(),
	}

	c.hub.broadcast <- &BroadcastMessage{
		RoomID:  msg.RoomID,
		Message: notification,
		Sender:  c,
	}
}

// handleLeaveRoom unsubscribes client from a room's broadcasts
func (c *Client) handleLeaveRoom(msg *IncomingMessage) {
	if msg.RoomID == 0 {
		c.sendError("Room ID is required")
		return
	}

	c.hub.LeaveRoom(msg.RoomID, c)

	notification := &WSMessage{
		Type:      MessageTypeSystem,
		RoomID:    msg.RoomID,
		Content:   c.Username + " went offline",
		SenderID:  c.UserID,
		Sender:    "system",
		Timestamp: time.Now(),
	}

	c.hub.broadcast <- &BroadcastMessage{
		RoomID:  msg.RoomID,
		Message: notification,
		Sender:  c,
	}
}

// handleTyping broadcasts typing indicator
func (c *Client) handleTyping(msg *IncomingMessage) {
	wsMsg := &WSMessage{
		Type:      MessageTypeTyping,
		RoomID:    msg.RoomID,
		SenderID:  c.UserID,
		Sender:    c.Username,
		Timestamp: time.Now(),
	}

	c.hub.broadcast <- &BroadcastMessage{
		RoomID:  msg.RoomID,
		Message: wsMsg,
		Sender:  c,
	}
}

// handleStopTyping broadcasts stop-typing indicator
func (c *Client) handleStopTyping(msg *IncomingMessage) {
	wsMsg := &WSMessage{
		Type:      MessageTypeStopTyping,
		RoomID:    msg.RoomID,
		SenderID:  c.UserID,
		Sender:    c.Username,
		Timestamp: time.Now(),
	}

	c.hub.broadcast <- &BroadcastMessage{
		RoomID:  msg.RoomID,
		Message: wsMsg,
		Sender:  c,
	}
}

// sendError sends an error message back to the client
func (c *Client) sendError(message string) {
	errMsg := &WSMessage{
		Type:      MessageTypeError,
		Content:   message,
		Timestamp: time.Now(),
	}
	select {
	case c.send <- errMsg:
	default:
	}
}
