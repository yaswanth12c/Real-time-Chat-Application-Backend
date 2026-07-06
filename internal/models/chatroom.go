package models

import (
	"database/sql"
	"time"

	"github.com/yaswa/go-chat-backend/internal/database"
)

type ChatRoom struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   int64     `json:"created_by"`
	IsPrivate   bool      `json:"is_private"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChatRoomMember struct {
	RoomID   int64     `json:"room_id"`
	UserID   int64     `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type CreateRoomInput struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
}

type UpdateRoomInput struct {
	Name        string `json:"name" binding:"omitempty,min=2,max=100"`
	Description string `json:"description"`
}

func CreateRoom(input *CreateRoomInput, createdBy int64) (*ChatRoom, error) {
	result, err := database.DB.Exec(
		"INSERT INTO chat_rooms (name, description, created_by, is_private) VALUES (?, ?, ?, ?)",
		input.Name, input.Description, createdBy, input.IsPrivate,
	)
	if err != nil {
		return nil, err
	}

	roomID, _ := result.LastInsertId()

	// Add creator as owner
	_, err = database.DB.Exec(
		"INSERT INTO chat_room_members (room_id, user_id, role) VALUES (?, ?, 'owner')",
		roomID, createdBy,
	)
	if err != nil {
		return nil, err
	}

	return GetRoomByID(roomID)
}

func GetRoomByID(id int64) (*ChatRoom, error) {
	room := &ChatRoom{}
	var description sql.NullString

	err := database.DB.QueryRow(
		"SELECT id, name, description, created_by, is_private, created_at, updated_at FROM chat_rooms WHERE id = ?",
		id,
	).Scan(&room.ID, &room.Name, &description, &room.CreatedBy, &room.IsPrivate, &room.CreatedAt, &room.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if description.Valid {
		room.Description = description.String
	}
	return room, err
}

func ListUserRooms(userID int64) ([]ChatRoom, error) {
	rows, err := database.DB.Query(
		`SELECT cr.id, cr.name, cr.description, cr.created_by, cr.is_private, cr.created_at, cr.updated_at
		 FROM chat_rooms cr
		 INNER JOIN chat_room_members crm ON cr.id = crm.room_id
		 WHERE crm.user_id = ?
		 ORDER BY cr.updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []ChatRoom
	for rows.Next() {
		var room ChatRoom
		var description sql.NullString
		if err := rows.Scan(&room.ID, &room.Name, &description, &room.CreatedBy,
			&room.IsPrivate, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, err
		}
		if description.Valid {
			room.Description = description.String
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func ListPublicRooms() ([]ChatRoom, error) {
	rows, err := database.DB.Query(
		`SELECT id, name, description, created_by, is_private, created_at, updated_at
		 FROM chat_rooms WHERE is_private = FALSE
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []ChatRoom
	for rows.Next() {
		var room ChatRoom
		var description sql.NullString
		if err := rows.Scan(&room.ID, &room.Name, &description, &room.CreatedBy,
			&room.IsPrivate, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, err
		}
		if description.Valid {
			room.Description = description.String
		}
		rooms = append(rooms, room)
	}
	return rooms, nil
}

func UpdateRoom(roomID int64, input *UpdateRoomInput) (*ChatRoom, error) {
	if input.Name != "" {
		if _, err := database.DB.Exec("UPDATE chat_rooms SET name = ? WHERE id = ?", input.Name, roomID); err != nil {
			return nil, err
		}
	}
	if input.Description != "" {
		if _, err := database.DB.Exec("UPDATE chat_rooms SET description = ? WHERE id = ?", input.Description, roomID); err != nil {
			return nil, err
		}
	}
	return GetRoomByID(roomID)
}

func DeleteRoom(roomID int64) error {
	_, err := database.DB.Exec("DELETE FROM chat_rooms WHERE id = ?", roomID)
	return err
}

func AddMember(roomID, userID int64, role string) error {
	_, err := database.DB.Exec(
		"INSERT INTO chat_room_members (room_id, user_id, role) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE role = role",
		roomID, userID, role,
	)
	return err
}

func RemoveMember(roomID, userID int64) error {
	_, err := database.DB.Exec(
		"DELETE FROM chat_room_members WHERE room_id = ? AND user_id = ?",
		roomID, userID,
	)
	return err
}

func GetRoomMembers(roomID int64) ([]ChatRoomMember, error) {
	rows, err := database.DB.Query(
		`SELECT crm.room_id, crm.user_id, u.username, crm.role, crm.joined_at
		 FROM chat_room_members crm
		 INNER JOIN users u ON crm.user_id = u.id
		 WHERE crm.room_id = ?
		 ORDER BY crm.joined_at ASC`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ChatRoomMember
	for rows.Next() {
		var m ChatRoomMember
		if err := rows.Scan(&m.RoomID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func IsMember(roomID, userID int64) (bool, error) {
	var count int
	err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM chat_room_members WHERE room_id = ? AND user_id = ?",
		roomID, userID,
	).Scan(&count)
	return count > 0, err
}

func IsRoomOwner(roomID, userID int64) (bool, error) {
	var count int
	err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM chat_room_members WHERE room_id = ? AND user_id = ? AND role = 'owner'",
		roomID, userID,
	).Scan(&count)
	return count > 0, err
}
