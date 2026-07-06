package models

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yaswa/go-chat-backend/internal/database"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	AvatarURL    string    `json:"avatar_url"`
	IsOnline     bool      `json:"is_online"`
	LastSeen     time.Time `json:"last_seen"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterInput struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdateProfileInput struct {
	Username  string `json:"username" binding:"omitempty,min=3,max=50"`
	Email     string `json:"email" binding:"omitempty,email"`
	AvatarURL string `json:"avatar_url"`
}

func CreateUser(input *RegisterInput) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	result, err := database.DB.Exec(
		"INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
		input.Username, input.Email, string(hashedPassword),
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return GetUserByID(id)
}

func GetUserByID(id int64) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, username, email, password_hash, avatar_url, is_online, last_seen, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AvatarURL,
		&user.IsOnline, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, username, email, password_hash, avatar_url, is_online, last_seen, created_at, updated_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AvatarURL,
		&user.IsOnline, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func GetUserByEmail(email string) (*User, error) {
	user := &User{}
	err := database.DB.QueryRow(
		"SELECT id, username, email, password_hash, avatar_url, is_online, last_seen, created_at, updated_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.AvatarURL,
		&user.IsOnline, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func UpdateUserProfile(userID int64, input *UpdateProfileInput) (*User, error) {
	if input.Username != "" {
		if _, err := database.DB.Exec("UPDATE users SET username = ? WHERE id = ?", input.Username, userID); err != nil {
			return nil, err
		}
	}
	if input.Email != "" {
		if _, err := database.DB.Exec("UPDATE users SET email = ? WHERE id = ?", input.Email, userID); err != nil {
			return nil, err
		}
	}
	if input.AvatarURL != "" {
		if _, err := database.DB.Exec("UPDATE users SET avatar_url = ? WHERE id = ?", input.AvatarURL, userID); err != nil {
			return nil, err
		}
	}
	return GetUserByID(userID)
}

func SetUserOnlineStatus(userID int64, online bool) error {
	_, err := database.DB.Exec(
		"UPDATE users SET is_online = ?, last_seen = NOW() WHERE id = ?",
		online, userID,
	)
	return err
}

func CheckPassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}
