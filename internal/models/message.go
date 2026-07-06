package models

import (
	"time"

	"github.com/yaswa/go-chat-backend/internal/database"
)

type Message struct {
	ID          int64     `json:"id"`
	RoomID      int64     `json:"room_id"`
	SenderID    int64     `json:"sender_id"`
	SenderName  string    `json:"sender_name"`
	Content     string    `json:"content"`
	MessageType string    `json:"message_type"`
	CreatedAt   time.Time `json:"created_at"`
}

func SaveMessage(roomID, senderID int64, content, messageType string) (*Message, error) {
	if messageType == "" {
		messageType = "text"
	}

	result, err := database.DB.Exec(
		"INSERT INTO messages (room_id, sender_id, content, message_type) VALUES (?, ?, ?, ?)",
		roomID, senderID, content, messageType,
	)
	if err != nil {
		return nil, err
	}

	msgID, _ := result.LastInsertId()
	return GetMessageByID(msgID)
}

func GetMessageByID(id int64) (*Message, error) {
	msg := &Message{}
	err := database.DB.QueryRow(
		`SELECT m.id, m.room_id, m.sender_id, u.username, m.content, m.message_type, m.created_at
		 FROM messages m
		 INNER JOIN users u ON m.sender_id = u.id
		 WHERE m.id = ?`,
		id,
	).Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderName, &msg.Content, &msg.MessageType, &msg.CreatedAt)

	return msg, err
}

func GetMessagesByRoom(roomID int64, limit, offset int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := database.DB.Query(
		`SELECT m.id, m.room_id, m.sender_id, u.username, m.content, m.message_type, m.created_at
		 FROM messages m
		 INNER JOIN users u ON m.sender_id = u.id
		 WHERE m.room_id = ?
		 ORDER BY m.created_at DESC
		 LIMIT ? OFFSET ?`,
		roomID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderName,
			&msg.Content, &msg.MessageType, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func GetRoomMessageCount(roomID int64) (int, error) {
	var count int
	err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM messages WHERE room_id = ?",
		roomID,
	).Scan(&count)
	return count, err
}
