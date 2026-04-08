package main

import (
	"fmt"
	"log"
	"os"

	"github.com/deriva-inc/keyper-go/config"
	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/router"
	"github.com/deriva-inc/keyper-go/utils/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	// For now, we will hardcode the port.
	// In the next step, we would load this from config/config.go
	// STEP 1: Load Configuration
	logger := logger.New()
	cfg, err := config.Load()
	if err != nil {
		logger.Error("❌ Failed to load configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("🎛️ Configuration loaded successfully")
	port := cfg.HTTP.Port

	// STEP 2: Connect to Database
	logger.Info("🔗 Connecting to database at %s...", cfg.DB.DSN)
	database, err := db.Connect(cfg.DB.DSN)
	if err != nil {
		logger.Error("❌ Failed to connect to database", "error", err)
		os.Exit(1)
	}
	logger.Info("🐘 Database connection established")

	// STEP 3: Initialize Gin Engine
	gin.SetMode(cfg.HTTP.Mode) // Use gin.DebugMode for development
	engine := gin.Default()    // gin.Default() comes with Logger and Recovery middleware

	// STEP 4: Setup Routes
	router.SetupRoutes(engine, database)

	// For this example, let's create a simple health check route
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	// STEP 5: Start Server
	serverAddr := fmt.Sprintf(":%s", port)
	// TODO: For production, change the log to actual URL of the server.
	log.Printf("✅ Server listening on http://localhost%s", serverAddr)

	if err := engine.Run(serverAddr); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
