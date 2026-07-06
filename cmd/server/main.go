package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yaswa/go-chat-backend/internal/auth"
	"github.com/yaswa/go-chat-backend/internal/config"
	"github.com/yaswa/go-chat-backend/internal/database"
	"github.com/yaswa/go-chat-backend/internal/handlers"
	ws "github.com/yaswa/go-chat-backend/internal/websocket"
)

func main() {
	// Load configuration
	config.Load()
	cfg := config.AppConfig

	// Initialize database connections
	database.InitMySQL()
	defer database.CloseMySQL()

	database.InitRedis()
	defer database.CloseRedis()

	// Initialize WebSocket hub
	hub := ws.NewHub()
	ws.ChatHub = hub
	go hub.Run()

	// Setup Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Serve frontend
	router.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})
	router.GET("/styles.css", func(c *gin.Context) {
		c.File("./web/styles.css")
	})
	router.GET("/app.js", func(c *gin.Context) {
		c.File("./web/app.js")
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":       "ok",
			"online_users": hub.GetOnlineCount(),
		})
	})

	// Public routes (no auth required)
	authRoutes := router.Group("/api/auth")
	{
		authRoutes.POST("/register", handlers.Register)
		authRoutes.POST("/login", handlers.Login)
		authRoutes.POST("/refresh", handlers.RefreshToken)
	}

	// Protected routes (auth required)
	api := router.Group("/api")
	api.Use(auth.AuthMiddleware())
	{
		// Auth
		api.POST("/auth/logout", handlers.Logout)

		// User profile
		api.GET("/users/me", handlers.GetProfile)
		api.PUT("/users/me", handlers.UpdateProfile)
		api.GET("/users/:id", handlers.GetUserByID)

		// Chat rooms
		api.POST("/rooms", handlers.CreateRoom)
		api.GET("/rooms", handlers.ListRooms)
		api.GET("/rooms/:id", handlers.GetRoom)
		api.PUT("/rooms/:id", handlers.UpdateRoom)
		api.DELETE("/rooms/:id", handlers.DeleteRoom)
		api.POST("/rooms/:id/join", handlers.JoinRoom)
		api.POST("/rooms/:id/leave", handlers.LeaveRoom)
		api.GET("/rooms/:id/members", handlers.GetRoomMembers)

		// Messages
		api.GET("/rooms/:id/messages", handlers.GetMessages)
	}

	// WebSocket endpoint
	router.GET("/ws", ws.ServeWS(hub))

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
