package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/yaswa/go-chat-backend/internal/database"
)

// Hub maintains active clients and broadcasts messages to rooms
type Hub struct {
	// Registered clients mapped by user ID
	clients map[int64]*Client

	// Room subscriptions: roomID -> set of client pointers
	rooms map[int64]map[*Client]bool

	// Channel for registering clients
	register chan *Client

	// Channel for unregistering clients
	unregister chan *Client

	// Channel for broadcasting messages to rooms
	broadcast chan *BroadcastMessage

	// Mutex for thread-safe map access
	mu sync.RWMutex
}

var ChatHub *Hub

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int64]*Client),
		rooms:      make(map[int64]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
	}
}

func (h *Hub) Run() {
	// Start Redis subscriber in background
	go h.subscribeRedis()

	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.UserID] = client
	log.Printf("Client registered: %s (ID: %d)", client.Username, client.UserID)

	// Mark user online
	database.SetUserOnline(client.UserID)
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.UserID]; ok {
		delete(h.clients, client.UserID)
		close(client.send)

		// Remove from all rooms
		for roomID, members := range h.rooms {
			if _, ok := members[client]; ok {
				delete(members, client)
				if len(members) == 0 {
					delete(h.rooms, roomID)
				}
			}
		}

		// Mark user offline
		database.SetUserOffline(client.UserID)
		log.Printf("Client unregistered: %s (ID: %d)", client.Username, client.UserID)
	}
}

func (h *Hub) handleBroadcast(msg *BroadcastMessage) {
	// Publish to Redis for cross-instance delivery and let subscribeRedis handle the actual delivery to all clients
	channel := "chat_room"
	database.PublishMessage(channel, msg.Message)
}

// JoinRoom adds a client to a room's broadcast group
func (h *Hub) JoinRoom(roomID int64, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	h.rooms[roomID][client] = true
	client.rooms[roomID] = true

	log.Printf("User %s joined room %d", client.Username, roomID)
}

// LeaveRoom removes a client from a room's broadcast group
func (h *Hub) LeaveRoom(roomID int64, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if members, ok := h.rooms[roomID]; ok {
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, roomID)
		}
	}
	delete(client.rooms, roomID)

	log.Printf("User %s left room %d", client.Username, roomID)
}

// subscribeRedis listens for messages from other server instances
func (h *Hub) subscribeRedis() {
	pubsub := database.Subscribe("chat_room")
	defer pubsub.Close()

	ch := pubsub.Channel()
	ctx := context.Background()
	_ = ctx

	for msg := range ch {
		var wsMsg WSMessage
		if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
			log.Printf("Failed to unmarshal Redis message: %v", err)
			continue
		}

		// Deliver to local clients in the room
		h.mu.RLock()
		if members, ok := h.rooms[wsMsg.RoomID]; ok {
			for client := range members {
				select {
				case client.send <- &wsMsg:
				default:
				}
			}
		}
		h.mu.RUnlock()
	}
}

// GetOnlineCount returns the number of connected clients
func (h *Hub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
