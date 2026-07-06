package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yaswa/go-chat-backend/internal/config"
)

var RedisClient *redis.Client

var ctx = context.Background()

func InitRedis() {
	cfg := config.AppConfig
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully")
}

// Session management

func StoreSession(userID int64, sessionID string, expiry time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return RedisClient.Set(ctx, key, userID, expiry).Err()
}

func GetSession(sessionID string) (int64, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	val, err := RedisClient.Get(ctx, key).Int64()
	if err != nil {
		return 0, err
	}
	return val, nil
}

func DeleteSession(sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return RedisClient.Del(ctx, key).Err()
}

// Online user tracking

func SetUserOnline(userID int64) error {
	key := "online_users"
	return RedisClient.SAdd(ctx, key, userID).Err()
}

func SetUserOffline(userID int64) error {
	key := "online_users"
	return RedisClient.SRem(ctx, key, userID).Err()
}

func GetOnlineUsers() ([]string, error) {
	key := "online_users"
	return RedisClient.SMembers(ctx, key).Result()
}

func IsUserOnline(userID int64) (bool, error) {
	key := "online_users"
	return RedisClient.SIsMember(ctx, key, userID).Result()
}

// Pub/Sub for message broadcasting across instances

func PublishMessage(channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return RedisClient.Publish(ctx, channel, data).Err()
}

func Subscribe(channel string) *redis.PubSub {
	return RedisClient.Subscribe(ctx, channel)
}

// Rate limiting

func CheckRateLimit(userID int64, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("rate_limit:%d", userID)

	count, err := RedisClient.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		RedisClient.Expire(ctx, key, window)
	}

	return count <= int64(limit), nil
}

func CloseRedis() {
	if RedisClient != nil {
		RedisClient.Close()
		log.Println("Redis connection closed")
	}
}
