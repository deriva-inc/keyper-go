package handlers

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateRecoveryKey_Format(t *testing.T) {
	key, err := GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("GenerateRecoveryKey returned error: %v", err)
	}

	if !strings.HasPrefix(key, "keyper-recovery-") {
		t.Errorf("expected recovery key to start with 'keyper-recovery-', got %s", key)
	}

	parts := strings.Split(key, "-")
	// "keyper", "recovery", word1, word2, word3, number, word4 -> 7 parts
	if len(parts) < 6 {
		t.Errorf("expected key to have at least 6 segments, got %d segments in %s", len(parts), key)
	}
}

func TestGenerateRecoveryKey_Entropy(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		key, err := GenerateRecoveryKey()
		if err != nil {
			t.Fatalf("GenerateRecoveryKey failed at iteration %d: %v", i, err)
		}
		if seen[key] {
			t.Fatalf("duplicate recovery key generated: %s", key)
		}
		seen[key] = true
	}
}

func TestGenerateJWT_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	token, err := GenerateJWT(userID, 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if token == "" {
		t.Errorf("expected non-empty token string")
	}
}

func TestGenerateJWT_MissingSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	_, err := GenerateJWT(userID, 1*time.Hour)
	if err == nil {
		t.Fatalf("expected error when JWT_SECRET is unset, got nil")
	}
}
