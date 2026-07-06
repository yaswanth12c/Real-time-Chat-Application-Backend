package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort     string
	MySQLHost      string
	MySQLPort      string
	MySQLUser      string
	MySQLPassword  string
	MySQLDatabase  string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	JWTSecret      string
	AccessExpiry   time.Duration
	RefreshExpiry  time.Duration
}

var AppConfig *Config

func Load() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	accessExpiry, _ := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m"))
	refreshExpiry, _ := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h"))

	AppConfig = &Config{
		ServerPort:    getEnv("SERVER_PORT", "8080"),
		MySQLHost:     getEnv("MYSQL_HOST", "localhost"),
		MySQLPort:     getEnv("MYSQL_PORT", "3306"),
		MySQLUser:     getEnv("MYSQL_USER", "root"),
		MySQLPassword: getEnv("MYSQL_PASSWORD", ""),
		MySQLDatabase: getEnv("MYSQL_DATABASE", "chatapp"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
		JWTSecret:     getEnv("JWT_SECRET", "default-secret-change-me"),
		AccessExpiry:  accessExpiry,
		RefreshExpiry: refreshExpiry,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
