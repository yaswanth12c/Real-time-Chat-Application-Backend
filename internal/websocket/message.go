package websocket

import "time"

// WSMessage represents a WebSocket message envelope
type WSMessage struct {
	Type      string      `json:"type"`
	RoomID    int64       `json:"room_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	SenderID  int64       `json:"sender_id,omitempty"`
	Sender    string      `json:"sender,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Message types
const (
	MessageTypeChat       = "chat_message"
	MessageTypeJoinRoom   = "join_room"
	MessageTypeLeaveRoom  = "leave_room"
	MessageTypeTyping     = "typing"
	MessageTypeStopTyping = "stop_typing"
	MessageTypeUserOnline = "user_online"
	MessageTypeUserOffline = "user_offline"
	MessageTypeError      = "error"
	MessageTypeSystem     = "system"
)

// IncomingMessage represents a message from the client
type IncomingMessage struct {
	Type    string `json:"type"`
	RoomID  int64  `json:"room_id"`
	Content string `json:"content"`
}

// BroadcastMessage wraps a message with target room info for the hub
type BroadcastMessage struct {
	RoomID  int64
	Message *WSMessage
	Sender  *Client
}
