// test-backend/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	leviathan "leviathan-bridge/lev-sdks"

	"github.com/gin-gonic/gin"
)

// Simple user model for testing
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Mock data
var users = []User{
	{ID: 1, Name: "Alice", Role: "Admin"},
	{ID: 2, Name: "Bob", Role: "User"},
	{ID: 3, Name: "Charlie", Role: "Manager"},
}

func main() {
	log.Println("Starting Test Backend...")

	// Initialize Leviathan SDK (will load lev.yaml automatically)
	sdk, err := leviathan.NewBackendSDK("")
	if err != nil {
		log.Fatalf("Failed to initialize Leviathan SDK: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Auto-register with Leviathan Agent
	if err := sdk.AutoRegister(ctx); err != nil {
		log.Printf("Warning: Failed to register with Leviathan Agent: %v", err)
		log.Println("Continuing without Leviathan registration...")
	} else {
		log.Println("✅ Successfully registered with Leviathan Agent")
		// Start heartbeat
		if err := sdk.StartAutoHeartbeat(ctx, time.Minute); err != nil {
			log.Printf("Warning: Failed to start heartbeat: %v", err)
		} else {
			log.Println("✅ Heartbeat started")
		}
	}

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Health check endpoint (required by Leviathan Agent)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   "test-api",
		})
	})

	// API endpoints
	r.GET("/users", getUsers)
	r.GET("/users/:id", getUser)
	r.POST("/users", createUser)
	r.GET("/info", getInfo)

	// Get port from config
	config := sdk.GetConfig()
	port := 8080
	if config != nil {
		port = config.Port
	}

	// Start server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		log.Println("Server stopped")
	}()

	log.Printf("Test Backend running on port %d", port)
	log.Println("Available endpoints:")
	log.Println("  GET  /healthz     - Health check")
	log.Println("  GET  /users       - List all users")
	log.Println("  GET  /users/:id   - Get user by ID")
	log.Println("  POST /users       - Create new user")
	log.Println("  GET  /info        - Service information")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Handler functions
func getUsers(c *gin.Context) {
	c.JSON(200, gin.H{
		"users": users,
		"count": len(users),
	})
}

func getUser(c *gin.Context) {
	id := c.Param("id")

	for _, user := range users {
		if fmt.Sprintf("%d", user.ID) == id {
			c.JSON(200, user)
			return
		}
	}

	c.JSON(404, gin.H{"error": "User not found"})
}

func createUser(c *gin.Context) {
	var newUser User
	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Generate new ID
	maxID := 0
	for _, user := range users {
		if user.ID > maxID {
			maxID = user.ID
		}
	}
	newUser.ID = maxID + 1

	users = append(users, newUser)
	c.JSON(201, newUser)
}

func getInfo(c *gin.Context) {
	c.JSON(200, gin.H{
		"service":   "test-api",
		"version":   "1.0.0",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(time.Now()).String(),
	})
}
