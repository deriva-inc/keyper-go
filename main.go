package main

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

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

// isTerminal checks if standard input is an interactive terminal session.
func isTerminal() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func handleCLICommands() bool {
	if len(os.Args) < 2 {
		return false
	}

	cmd := os.Args[1]

	switch cmd {
	case "-v", "--version", "version":
		fmt.Printf("Keyper %s (commit: %s, built: %s)\n", config.Version, config.GitCommit, config.BuildDate)
		os.Exit(0)

	case "setup", "init":
		_, err := config.InteractiveSetup()
		if err != nil {
			log.Fatalf("❌ Setup failed: %v", err)
		}
		log.Println("✅ Setup completed. Start Keyper with: keyper")
		os.Exit(0)

	case "config":
		if len(os.Args) >= 4 && os.Args[2] == "set" {
			key := os.Args[3]
			val := os.Args[4]
			if err := config.SetConfigValue(key, val); err != nil {
				log.Fatalf("❌ Failed to set config '%s': %v", key, err)
			}
			log.Printf("✅ Successfully updated '%s' in config.yaml\n", key)
			os.Exit(0)
		} else if len(os.Args) >= 3 && os.Args[2] == "show" {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("❌ Failed to load config: %v", err)
			}
			fmt.Println("\nCurrent Keyper Configuration:")
			fmt.Printf("  HTTP Port:   %s\n", cfg.HTTP.Port)
			fmt.Printf("  HTTP Mode:   %s\n", cfg.HTTP.Mode)
			fmt.Printf("  Database:    %s\n", cfg.DB.DSN)
			fmt.Printf("  Pepper:      %s\n", "[SET - hidden]")
			fmt.Printf("  JWT Secret:  %s\n", "[SET - hidden]")
			os.Exit(0)
		} else {
			fmt.Println("Usage:")
			fmt.Println("  keyper config set <key> <value>   (e.g. keyper config set db.dsn \"postgres://...\")")
			fmt.Println("  keyper config show                (display current config summary)")
			os.Exit(1)
		}
	}

	return false
}

func main() {
	// Handle CLI subcommands (setup, config set, version)
	handleCLICommands()

	appLogger := logger.New()

	// STEP 1: Load Configuration from config.yaml
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			if isTerminal() {
				fmt.Print("\n⚠️ config.yaml was not found. Would you like to run the setup wizard now? [Y/n]: ")
				reader := bufio.NewReader(os.Stdin)
				ans, _ := reader.ReadString('\n')
				ans = strings.TrimSpace(strings.ToLower(ans))
				if ans == "" || ans == "y" || ans == "yes" {
					cfg, err = config.InteractiveSetup()
					if err != nil {
						appLogger.Error("❌ Setup failed", "error", err)
						os.Exit(1)
					}
				} else {
					fmt.Println("Exiting. You can run setup later with: keyper setup")
					os.Exit(1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "\n=======================================================\n")
				fmt.Fprintf(os.Stderr, "❌ Configuration Error: config.yaml not found!\n\n")
				fmt.Fprintf(os.Stderr, "Keyper is config-driven and requires a valid config.yaml.\n\n")
				fmt.Fprintf(os.Stderr, "How to configure:\n")
				fmt.Fprintf(os.Stderr, "  1. Run the interactive setup wizard:\n")
				fmt.Fprintf(os.Stderr, "     keyper setup\n")
				fmt.Fprintf(os.Stderr, "     (Docker: docker run -it -v $(pwd)/config:/app/config keyper:latest setup)\n\n")
				fmt.Fprintf(os.Stderr, "  2. Or mount an existing config directory containing config.yaml:\n")
				fmt.Fprintf(os.Stderr, "     docker run -v /path/to/config:/app/config keyper:latest\n")
				fmt.Fprintf(os.Stderr, "=======================================================\n\n")
				os.Exit(1)
			}
		} else {
			appLogger.Error("❌ Failed to load configuration", "error", err)
			os.Exit(1)
		}
	}

	// Validate required configurations in config.yaml
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "\n=======================================================\n❌ Configuration Error in config.yaml:\n%v\n\nRun 'keyper setup' or edit config.yaml to configure missing values.\n=======================================================\n\n", err)
		os.Exit(1)
	}

	appLogger.Info("🎛️ Configuration loaded successfully from config.yaml")

	// Enable hot-reloading on config.yaml changes
	config.WatchConfig(func() {
		appLogger.Info("🔄 Detected changes in config.yaml - configuration reloaded")
	})

	port := cfg.HTTP.Port

	// STEP 2: Run Database Migrations
	appLogger.Info("🏃 Checking for database migrations...")

	sourceInstance, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		appLogger.Error("❌ Failed to create migration source from embedded files: %v", err)
		os.Exit(1)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceInstance, cfg.DB.DSN)
	if err != nil {
		appLogger.Error("❌ Failed to create migrate instance: %v", err)
		os.Exit(1)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		appLogger.Error("❌ Failed to apply migrations: %v", err)
		os.Exit(1)
	} else if err == migrate.ErrNoChange {
		appLogger.Info("✅ No new migrations to apply. Database is up-to-date.")
	} else {
		appLogger.Info("✅ Database migration applied successfully.")
	}

	// STEP 3: Connect to Database
	appLogger.Info("🔗 Connecting to database at %s...", cfg.DB.DSN)
	database, err := db.Connect(cfg.DB.DSN)
	if err != nil {
		appLogger.Error("❌ Failed to connect to database", "error", err)
		os.Exit(1)
	}
	appLogger.Info("🐘 Database connection established")

	// STEP 4: Initialize Gin Engine
	gin.SetMode(cfg.HTTP.Mode)
	engine := gin.Default()

	// Add CORS middleware
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-User-Id", "X-Profile-Id"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// STEP 5: Setup Routes
	router.SetupRoutes(engine, database)

	// STEP 6: Start Server
	serverAddr := fmt.Sprintf(":%s", port)
	log.Printf("✅ Server listening on http://localhost%s", serverAddr)

	if err := engine.Run(serverAddr); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
