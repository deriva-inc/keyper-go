package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/deriva-inc/keyper-go/config"
	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/router"
	"github.com/deriva-inc/keyper-go/utils/logger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	// -- Migration Imports --
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

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

	// STEP 2: Run Database Migrations (The core logic is here)
	logger.Info("🏃 Checking for database migrations...")

	// Create a new migration "source" from our embedded filesystem.
	sourceInstance, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		logger.Error("❌ Failed to create migration source from embedded files: %v", err)
		os.Exit(1)
	}

	// Create a new migrate instance.
	m, err := migrate.NewWithSourceInstance("iofs", sourceInstance, cfg.DB.DSN)
	if err != nil {
		logger.Error("❌ Failed to create migrate instance: %v", err)
		os.Exit(1)
	}

	// Run the "Up" migrations.
	// This command is idempotent. It checks the 'schema_migrations' table
	// in the DB and only applies migrations that haven't been run yet.
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Error("❌ Failed to apply migrations: %v", err)
		os.Exit(1)
	} else if err == migrate.ErrNoChange {
		logger.Info("✅ No new migrations to apply. Database is up-to-date.")
	} else {
		logger.Info("✅ Database migration applied successfully.")
	}

	// STEP 3: Connect to Database
	logger.Info("🔗 Connecting to database at %s...", cfg.DB.DSN)
	database, err := db.Connect(cfg.DB.DSN)
	if err != nil {
		logger.Error("❌ Failed to connect to database", "error", err)
		os.Exit(1)
	}
	logger.Info("🐘 Database connection established")

	// STEP 4: Initialize Gin Engine
	gin.SetMode(cfg.HTTP.Mode) // Use gin.DebugMode for development
	engine := gin.Default()    // gin.Default() comes with Logger and Recovery middleware

	// Add CORS middleware
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "x-user-id"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// STEP 5: Setup Routes
	router.SetupRoutes(engine, database)

	// For this example, let's create a simple health check route
	engine.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	// STEP 6: Start Server
	serverAddr := fmt.Sprintf(":%s", port)
	// TODO: For production, change the log to actual URL of the server.
	log.Printf("✅ Server listening on http://localhost%s", serverAddr)

	if err := engine.Run(serverAddr); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
