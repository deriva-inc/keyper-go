package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Version info populated at build time via -ldflags
var (
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

// ErrConfigNotFound indicates that no configuration file was found.
var ErrConfigNotFound = errors.New("configuration file config.yaml not found")

// Config holds all the configuration for the application.
type Config struct {
	HTTP     HTTPConfig     `mapstructure:"http"`
	DB       DBConfig       `mapstructure:"db"`
	Security SecurityConfig `mapstructure:"security"`
}

// HTTPConfig holds the configuration for the HTTP server.
type HTTPConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DBConfig holds the configuration for the database.
type DBConfig struct {
	DSN string `mapstructure:"dsn"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	Pepper    string `mapstructure:"pepper"`
	JWTSecret string `mapstructure:"jwt_secret"`
}

// GenerateRandomHex produces a cryptographically secure hex string of given byte length.
func GenerateRandomHex(bytesLen int) (string, error) {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Validate checks whether all required configuration variables are set and not placeholder values.
func (c *Config) Validate() error {
	var missing []string

	if c.DB.DSN == "" || c.DB.DSN == "DSN_PLACEHOLDER_FROM_ENV" {
		missing = append(missing, "db.dsn: PostgreSQL connection string (e.g. postgres://user:password@localhost:5432/keyper_db?sslmode=disable)")
	}

	if c.Security.JWTSecret == "" || c.Security.JWTSecret == "JWT_SECRET_PLACEHOLDER" {
		missing = append(missing, "security.jwt_secret: secret key for signing session tokens")
	}

	if c.Security.Pepper == "" || c.Security.Pepper == "PEPPER_PLACEHOLDER_FROM_ENV" {
		missing = append(missing, "security.pepper: global secret pepper for cryptographic hashing")
	}

	if len(missing) > 0 {
		return fmt.Errorf("the following required configurations are missing or unconfigured in config.yaml:\n  - %s",
			strings.Join(missing, "\n  - "))
	}

	return nil
}

// GetConfigPath returns the directory path where config.yaml is or should be located.
func GetConfigDir() string {
	candidateDirs := []string{
		"./config",
		"/app/config",
		"/config",
		".",
	}

	for _, dir := range candidateDirs {
		if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil {
			return dir
		}
	}

	// Default to ./config
	return "./config"
}

// Save writes the in-memory configuration struct to config.yaml.
func Save(cfg *Config, targetDir string) error {
	if targetDir == "" {
		targetDir = GetConfigDir()
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", targetDir, err)
	}

	configFilePath := filepath.Join(targetDir, "config.yaml")

	viper.Set("http.port", cfg.HTTP.Port)
	viper.Set("http.mode", cfg.HTTP.Mode)
	viper.Set("db.dsn", cfg.DB.DSN)
	viper.Set("security.pepper", cfg.Security.Pepper)
	viper.Set("security.jwt_secret", cfg.Security.JWTSecret)

	if err := viper.WriteConfigAs(configFilePath); err != nil {
		// If already exists, use WriteConfig
		if os.IsExist(err) || strings.Contains(err.Error(), "already exists") {
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
			}
		} else {
			return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
		}
	}

	return nil
}

// SetConfigValue updates a single configuration key in config.yaml.
func SetConfigValue(key, value string) error {
	viper.Set(key, value)
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = filepath.Join(GetConfigDir(), "config.yaml")
		return viper.WriteConfigAs(configFile)
	}
	return viper.WriteConfig()
}

// InteractiveSetup guides the user through CLI prompts to generate config.yaml.
func InteractiveSetup() (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=======================================================")
	fmt.Println("🔧 Keyper Configuration Setup Wizard")
	fmt.Println("=======================================================")
	fmt.Println("This wizard will help you configure Keyper and generate config.yaml.")
	fmt.Println()

	// 1. Port
	fmt.Print("Enter HTTP Port [default: 8085]: ")
	portInput, _ := reader.ReadString('\n')
	port := strings.TrimSpace(portInput)
	if port == "" {
		port = "8085"
	}

	// 2. Gin Mode
	fmt.Print("Enter Server Mode (release/debug) [default: release]: ")
	modeInput, _ := reader.ReadString('\n')
	mode := strings.TrimSpace(modeInput)
	if mode == "" {
		mode = "release"
	}

	// 3. Database DSN
	fmt.Print("Enter Database DSN (e.g. postgres://user:pass@localhost:5432/keyper_db?sslmode=disable): ")
	dsnInput, _ := reader.ReadString('\n')
	dsn := strings.TrimSpace(dsnInput)
	for dsn == "" {
		fmt.Print("⚠️ Database DSN cannot be empty. Please enter DSN: ")
		dsnInput, _ = reader.ReadString('\n')
		dsn = strings.TrimSpace(dsnInput)
	}

	// 4. Cryptographic Secrets
	fmt.Println("\nGenerating high-entropy cryptographic secrets...")
	pepper, err := GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate security pepper: %w", err)
	}

	jwtSecret, err := GenerateRandomHex(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
	}
	fmt.Println("✓ Generated 256-bit security pepper")
	fmt.Println("✓ Generated 256-bit JWT secret")

	cfg := &Config{
		HTTP: HTTPConfig{
			Port: port,
			Mode: mode,
		},
		DB: DBConfig{
			DSN: dsn,
		},
		Security: SecurityConfig{
			Pepper:    pepper,
			JWTSecret: jwtSecret,
		},
	}

	configDir := GetConfigDir()
	if err := Save(cfg, configDir); err != nil {
		return nil, err
	}

	fmt.Printf("\n✅ Configuration successfully written to: %s/config.yaml\n", configDir)
	fmt.Println("=======================================================")

	return cfg, nil
}

// WatchConfig enables hot-reloading when config.yaml is edited at runtime.
func WatchConfig(onChange func()) {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		if onChange != nil {
			onChange()
		}
	})
}

// Load reads configuration strictly from config.yaml.
func Load() (*Config, error) {
	viper.SetDefault("http.port", "8085")
	viper.SetDefault("http.mode", "release")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/app/config")
	viper.AddConfigPath("/config")
	viper.AddConfigPath(".")

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Keep os environment variables in sync for legacy handlers if present
	if cfg.Security.JWTSecret != "" {
		_ = os.Setenv("JWT_SECRET", cfg.Security.JWTSecret)
	}
	if cfg.Security.Pepper != "" {
		_ = os.Setenv("PEPPER", cfg.Security.Pepper)
	}

	return &cfg, nil
}
