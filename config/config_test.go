package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_DefaultValues(t *testing.T) {
	cfg, err := Load()
	if err != nil && err != ErrConfigNotFound {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg != nil && cfg.HTTP.Port == "" {
		t.Errorf("expected default HTTP port, got empty string")
	}
}

func TestConfig_Validate_MissingAll(t *testing.T) {
	cfg := &Config{
		HTTP: HTTPConfig{Port: "8085", Mode: "release"},
		DB:   DBConfig{DSN: ""},
		Security: SecurityConfig{
			Pepper:    "",
			JWTSecret: "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for empty configs, got nil")
	}
}

func TestConfig_Validate_MissingDSN(t *testing.T) {
	cfg := &Config{
		HTTP: HTTPConfig{Port: "8085", Mode: "release"},
		DB:   DBConfig{DSN: ""},
		Security: SecurityConfig{
			Pepper:    "some-pepper",
			JWTSecret: "some-jwt-secret",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for missing DSN, got nil")
	}
}

func TestConfig_Validate_MissingJWTSecret(t *testing.T) {
	cfg := &Config{
		HTTP: HTTPConfig{Port: "8085", Mode: "release"},
		DB:   DBConfig{DSN: "postgres://localhost:5432/db"},
		Security: SecurityConfig{
			Pepper:    "some-pepper",
			JWTSecret: "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for missing JWTSecret, got nil")
	}
}

func TestConfig_Validate_MissingPepper(t *testing.T) {
	cfg := &Config{
		HTTP: HTTPConfig{Port: "8085", Mode: "release"},
		DB:   DBConfig{DSN: "postgres://localhost:5432/db"},
		Security: SecurityConfig{
			Pepper:    "",
			JWTSecret: "some-jwt-secret",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for missing Pepper, got nil")
	}
}

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		HTTP: HTTPConfig{Port: "8085", Mode: "release"},
		DB:   DBConfig{DSN: "postgres://user:pass@localhost:5432/keyper_db"},
		Security: SecurityConfig{
			Pepper:    "valid-pepper-string",
			JWTSecret: "valid-jwt-secret-string",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validation success, got error: %v", err)
	}
}

func TestGenerateRandomHex(t *testing.T) {
	hex1, err := GenerateRandomHex(16)
	if err != nil {
		t.Fatalf("failed to generate hex: %v", err)
	}
	if len(hex1) != 32 {
		t.Errorf("expected 32 hex chars for 16 bytes, got %d", len(hex1))
	}

	hex2, _ := GenerateRandomHex(16)
	if hex1 == hex2 {
		t.Errorf("expected random strings to differ")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		HTTP: HTTPConfig{Port: "9090", Mode: "debug"},
		DB:   DBConfig{DSN: "postgres://test:test@localhost:5432/test"},
		Security: SecurityConfig{
			Pepper:    "test-pepper",
			JWTSecret: "test-jwt-secret",
		},
	}

	if err := Save(cfg, tmpDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	savedFile := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(savedFile); err != nil {
		t.Fatalf("expected config.yaml to exist in temp dir: %v", err)
	}
}
