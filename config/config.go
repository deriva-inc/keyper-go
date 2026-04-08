package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds all the configuration for the application.
type Config struct {
	HTTP     HTTPConfig
	DB       DBConfig
	Security SecurityConfig
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
	Pepper string `mapstructure:"pepper"`
}

/*
* This function reads configuration from file and environment variables.
 */
func Load() (*Config, error) {
	// STEP 1: Setup Viper to read config from config/config.yaml file
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// Enable Viper to read Environment Variables
	// This allows you to override config values with env vars, which is great for production.
	// e.g., DB_DSN will override db.dsn
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		// Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			return nil, err
		}
		// Config file not found; ignore error if it's not essential
	}

	// Unmarshal the loaded configuration into our struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
