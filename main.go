package main

import (
	"fmt"
	"log"

	"github.com/deriva-inc/keyper-go/router"
	"github.com/gin-gonic/gin"
)

func main() {
	// For now, we will hardcode the port.
	// In the next step, we would load this from config/config.go
	port := "8080"

	// STEP 1: Initialize Gin Engine
	gin.SetMode(gin.ReleaseMode) // Use gin.DebugMode for development
	engine := gin.Default()      // gin.Default() comes with Logger and Recovery middleware

	// STEP 2: Setup Routes
	router.SetupRoutes(engine)

	// For this example, let's create a simple health check route
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	// STEP 3: Start Server
	serverAddr := fmt.Sprintf(":%s", port)
	// TODO: For production, change the log to actual URL of the server.
	log.Printf("✅ Server listening on http://localhost%s", serverAddr)

	if err := engine.Run(serverAddr); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
