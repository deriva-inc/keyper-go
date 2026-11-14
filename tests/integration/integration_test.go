package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/handlers"
	"github.com/deriva-inc/keyper-go/router"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

func setupTestApp(t *testing.T) (*gin.Engine, *db.DB, sqlmock.Sqlmock) {
	gin.SetMode(gin.TestMode)
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	database := db.NewFromDB(sqlxDB)

	r := gin.New()
	router.SetupRoutes(r, database)

	return r, database, mock
}

func TestIntegration_UserSignup_Success(t *testing.T) {
	r, _, mock := setupTestApp(t)

	userID := uuid.New()
	email := "newuser@example.com"

	// 1. SELECT * FROM users WHERE email = $1 -> User not found
	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnError(sqlmock.ErrCancelled) // simulates sql.ErrNoRows

	// 2. INSERT INTO users (...) VALUES (...) RETURNING ...
	mock.ExpectQuery("INSERT INTO users").
		WithArgs(email, "New User", "https://example.com/avatar.png", sqlmock.AnyArg(), "auth-salt", "enc-salt").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "avatar_url", "created_at", "updated_at"}).
			AddRow(userID, email, "New User", "https://example.com/avatar.png", time.Now(), time.Now()))

	body, _ := json.Marshal(map[string]string{
		"email":          email,
		"name":           "New User",
		"avatarUrl":      "https://example.com/avatar.png",
		"authKey":        "client-auth-key",
		"authSalt":       "auth-salt",
		"encryptionSalt": "enc-salt",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201 Created, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_UserSignup_Conflict(t *testing.T) {
	r, _, mock := setupTestApp(t)
	email := "existing@example.com"

	// SELECT returns existing user
	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(uuid.New(), email))

	body, _ := json.Marshal(map[string]string{
		"email":          email,
		"name":           "Existing",
		"avatarUrl":      "https://example.com/avatar.png",
		"authKey":        "auth-key",
		"authSalt":       "auth-salt",
		"encryptionSalt": "enc-salt",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409 Conflict, got %d", w.Code)
	}
}

func TestIntegration_UserLogin_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	r, _, mock := setupTestApp(t)
	email := "login@example.com"
	rawPasswordKey := "correct-client-auth-key"
	hashedKey, _ := bcrypt.GenerateFromPassword([]byte(rawPasswordKey), bcrypt.DefaultCost)
	userID := uuid.New()

	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "auth_key", "name", "created_at", "updated_at"}).
			AddRow(userID, email, string(hashedKey), "Login User", time.Now(), time.Now()))

	body, _ := json.Marshal(map[string]string{
		"email":   email,
		"authKey": rawPasswordKey,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_UserLogin_InvalidCredentials(t *testing.T) {
	r, _, mock := setupTestApp(t)
	email := "login@example.com"
	hashedKey, _ := bcrypt.GenerateFromPassword([]byte("real-key"), bcrypt.DefaultCost)

	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "auth_key"}).
			AddRow(uuid.New(), email, string(hashedKey)))

	body, _ := json.Marshal(map[string]string{
		"email":   email,
		"authKey": "wrong-key",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", w.Code)
	}
}

func TestIntegration_GetUserSalt(t *testing.T) {
	r, _, mock := setupTestApp(t)
	email := "salt@example.com"
	recSalt := "rec-salt-xyz"

	mock.ExpectQuery("SELECT auth_salt, encryption_salt, recovery_salt FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"auth_salt", "encryption_salt", "recovery_salt"}).
			AddRow("auth-salt-123", "enc-salt-456", recSalt))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/salt?email="+email, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", w.Code)
	}
}

func TestIntegration_RecoverySetup(t *testing.T) {
	r, _, mock := setupTestApp(t)
	email := "recovery@example.com"
	userID := uuid.New()

	mock.ExpectQuery("SELECT id, email FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(userID, email))

	mock.ExpectExec("UPDATE users").
		WithArgs(sqlmock.AnyArg(), "recovery-salt-123", userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{
		"email":        email,
		"recoverySalt": "recovery-salt-123",
		"recoveryHash": "client-derived-recovery-hash",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/recovery/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_RecoveryInitiate(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	r, _, mock := setupTestApp(t)
	email := "initiate@example.com"
	rawHash := "argon2-recovery-hash"
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(rawHash), bcrypt.DefaultCost)
	userID := uuid.New()

	mock.ExpectQuery("SELECT id, email, recovery_hash FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "recovery_hash"}).
			AddRow(userID, email, string(bcryptHash)))

	body, _ := json.Marshal(map[string]string{
		"email":        email,
		"recoveryHash": rawHash,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/recovery/initiate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestIntegration_ResetPasswordConfirm_And_RecoveryComplete(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	r, _, mock := setupTestApp(t)
	userID := uuid.New()
	email := "reset@example.com"

	restoreJWT, err := handlers.GenerateRestoreJWT(userID, email, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate restore token: %v", err)
	}

	// 1. Reset password confirm
	mock.ExpectQuery("SELECT id, email, auth_salt, encryption_salt FROM users WHERE id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "auth_salt", "encryption_salt"}).
			AddRow(userID, email, "auth-salt", "enc-salt"))

	confirmBody, _ := json.Marshal(map[string]string{
		"restoreJwt": restoreJWT,
	})

	wConfirm := httptest.NewRecorder()
	reqConfirm, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/reset-password/confirm", bytes.NewReader(confirmBody))
	reqConfirm.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wConfirm, reqConfirm)

	if wConfirm.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", wConfirm.Code)
	}

	// 2. Recovery complete
	mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(userID, email))

	mock.ExpectExec("UPDATE users").
		WithArgs(sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	completeBody, _ := json.Marshal(map[string]string{
		"restoreJwt": restoreJWT,
		"authKey":    "new-client-auth-key",
	})

	wComplete := httptest.NewRecorder()
	reqComplete, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/recovery/complete", bytes.NewReader(completeBody))
	reqComplete.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wComplete, reqComplete)

	if wComplete.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d, body: %s", wComplete.Code, wComplete.Body.String())
	}
}

