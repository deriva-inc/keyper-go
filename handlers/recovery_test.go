package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestGenerateAndValidateRestoreJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	email := "test@example.com"

	token, err := GenerateRestoreJWT(userID, email, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateRestoreJWT failed: %v", err)
	}

	claims, err := ParseAndValidateRestoreJWT(token)
	if err != nil {
		t.Fatalf("ParseAndValidateRestoreJWT failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, claims.UserID)
	}

	if claims.Email != email {
		t.Errorf("expected email %v, got %v", email, claims.Email)
	}

	if claims.Type != "restore" {
		t.Errorf("expected type 'restore', got %v", claims.Type)
	}
}

func TestExpiredRestoreJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	email := "test@example.com"

	token, err := GenerateRestoreJWT(userID, email, -1*time.Second)
	if err != nil {
		t.Fatalf("GenerateRestoreJWT failed: %v", err)
	}

	_, err = ParseAndValidateRestoreJWT(token)
	if err == nil {
		t.Fatalf("expected error for expired token, got nil")
	}
}

func TestTamperedRestoreJWT(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	userID := uuid.New()
	email := "test@example.com"

	token, err := GenerateRestoreJWT(userID, email, 15*time.Minute)
	if err != nil {
		t.Fatalf("GenerateRestoreJWT failed: %v", err)
	}

	os.Setenv("JWT_SECRET", "different-secret-key-1234567890123456")
	_, err = ParseAndValidateRestoreJWT(token)
	if err == nil {
		t.Fatalf("expected error for token with wrong secret, got nil")
	}
}

func TestExtractRestoreToken_FromBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)

	token, err := extractRestoreToken(c, "body-token-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "body-token-123" {
		t.Errorf("expected body-token-123, got %s", token)
	}
}

func TestExtractRestoreToken_FromBearerHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer header-token-456")

	token, err := extractRestoreToken(c, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "header-token-456" {
		t.Errorf("expected header-token-456, got %s", token)
	}
}

func TestExtractRestoreToken_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)

	_, err := extractRestoreToken(c, "")
	if err == nil {
		t.Fatalf("expected error when token is missing, got nil")
	}
}
