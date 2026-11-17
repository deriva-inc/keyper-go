package handlers

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
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

func setupSessionTestRouter(t *testing.T) (*gin.Engine, *db.DB, sqlmock.Sqlmock) {
	gin.SetMode(gin.TestMode)
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	database := db.NewFromDB(sqlxDB)

	r := gin.New()
	return r, database, mock
}

func TestGenerateRefreshToken(t *testing.T) {
	token1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	if len(token1) != 64 {
		t.Errorf("expected 64 hex characters, got %d", len(token1))
	}

	token2, _ := GenerateRefreshToken()
	if token1 == token2 {
		t.Errorf("expected unique tokens, got collision")
	}
}

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name         string
		ua           string
		expectedOS   string
		expectedBrow string
		expectedMfg  string
	}{
		{
			name:         "Chrome on macOS",
			ua:           "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			expectedOS:   "macOS",
			expectedBrow: "Chrome",
			expectedMfg:  "Apple",
		},
		{
			name:         "Safari on iOS",
			ua:           "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			expectedOS:   "iOS",
			expectedBrow: "Safari",
			expectedMfg:  "Apple",
		},
		{
			name:         "Firefox on Linux",
			ua:           "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/119.0",
			expectedOS:   "Linux",
			expectedBrow: "Firefox",
			expectedMfg:  "Generic PC",
		},
		{
			name:         "Empty UA",
			ua:           "",
			expectedOS:   "",
			expectedBrow: "",
			expectedMfg:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := ParseUserAgent(tc.ua)
			if tc.expectedOS == "" {
				if info.OS != nil || info.Browser != nil || info.Manufacturer != nil {
					t.Errorf("expected empty info, got %+v", info)
				}
				return
			}
			if info.OS == nil || *info.OS != tc.expectedOS {
				t.Errorf("OS mismatch: expected %s, got %v", tc.expectedOS, info.OS)
			}
			if info.Browser == nil || *info.Browser != tc.expectedBrow {
				t.Errorf("Browser mismatch: expected %s, got %v", tc.expectedBrow, info.Browser)
			}
			if info.Manufacturer == nil || *info.Manufacturer != tc.expectedMfg {
				t.Errorf("Manufacturer mismatch: expected %s, got %v", tc.expectedMfg, info.Manufacturer)
			}
		})
	}
}

func TestLogin_WithRememberMe_CreatesSession(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	r, database, mock := setupSessionTestRouter(t)
	r.POST("/login", Login(database))

	email := "test@example.com"
	rawKey := "secret-auth-key"
	hashedKey, _ := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	userID := uuid.New()

	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "auth_key", "name", "created_at", "updated_at"}).
			AddRow(userID, email, string(hashedKey), "Test User", time.Now(), time.Now()))

	mock.ExpectExec("INSERT INTO user_sessions").
		WithArgs(sqlmock.AnyArg(), userID, sqlmock.AnyArg(), sqlmock.AnyArg(), "Chrome", "macOS", "Apple").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]interface{}{
		"email":      email,
		"authKey":    rawKey,
		"rememberMe": true,
		"deviceInfo": map[string]string{
			"browser":      "Chrome",
			"os":           "macOS",
			"manufacturer": "Apple",
		},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.AccessToken == "" {
		t.Errorf("expected accessToken to be present")
	}
	if resp.Data.RefreshToken == "" {
		t.Errorf("expected refreshToken to be present when rememberMe is true")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	r, database, mock := setupSessionTestRouter(t)
	r.POST("/refresh", RefreshToken(database))

	sessionID := uuid.New()
	userID := uuid.New()
	oldRT := "valid-old-refresh-token"
	hashedOldRT := HashRefreshToken(oldRT)
	validExpiry := time.Now().Add(5 * 24 * time.Hour)

	mock.ExpectQuery("SELECT (.+) FROM user_sessions WHERE refresh_token = \\$1").
		WithArgs(hashedOldRT).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "refresh_token", "expiry_time", "browser", "os", "manufacturer", "creation_time", "last_active"}).
			AddRow(sessionID, userID, hashedOldRT, validExpiry, "Chrome", "macOS", "Apple", time.Now(), time.Now()))

	mock.ExpectExec("UPDATE user_sessions SET refresh_token = \\$1, last_active = NOW\\(\\) WHERE id = \\$2").
		WithArgs(sqlmock.AnyArg(), sessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{
		"refreshToken": oldRT,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.AccessToken == "" {
		t.Errorf("expected new access token")
	}
	if resp.Data.RefreshToken == "" || resp.Data.RefreshToken == oldRT {
		t.Errorf("expected rotated new refresh token, got %s", resp.Data.RefreshToken)
	}
}

func TestRefreshToken_Expired(t *testing.T) {
	r, database, mock := setupSessionTestRouter(t)
	r.POST("/refresh", RefreshToken(database))

	sessionID := uuid.New()
	userID := uuid.New()
	oldRT := "expired-refresh-token"
	hashedOldRT := HashRefreshToken(oldRT)
	expiredTime := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery("SELECT (.+) FROM user_sessions WHERE refresh_token = \\$1").
		WithArgs(hashedOldRT).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "refresh_token", "expiry_time", "browser", "os", "manufacturer", "creation_time", "last_active"}).
			AddRow(sessionID, userID, hashedOldRT, expiredTime, "Chrome", "macOS", "Apple", time.Now(), time.Now()))

	mock.ExpectExec("DELETE FROM user_sessions WHERE id = \\$1").
		WithArgs(sessionID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{
		"refreshToken": oldRT,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for expired token, got %d", w.Code)
	}
}

func TestLogout_Success(t *testing.T) {
	r, database, mock := setupSessionTestRouter(t)
	r.POST("/logout", Logout(database))

	token := "token-to-logout"
	hashedToken := HashRefreshToken(token)
	mock.ExpectExec("DELETE FROM user_sessions WHERE refresh_token = \\$1").
		WithArgs(hashedToken).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{
		"refreshToken": token,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestLogout_BearerToken_Success(t *testing.T) {
	r, database, mock := setupSessionTestRouter(t)
	userID := uuid.New()

	r.POST("/logout", func(c *gin.Context) {
		c.Set("userId", userID.String())
		Logout(database)(c)
	})

	mock.ExpectExec("DELETE FROM user_sessions WHERE id IN \\(SELECT id FROM user_sessions WHERE user_id = \\$1 ORDER BY last_active DESC LIMIT 1\\)").
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestRevokeAllSessions_Success(t *testing.T) {
	r, database, mock := setupSessionTestRouter(t)
	userID := uuid.New()

	r.POST("/logout-all", func(c *gin.Context) {
		c.Set("userId", userID.String())
		RevokeAllSessions(database)(c)
	})

	mock.ExpectExec("DELETE FROM user_sessions WHERE user_id = \\$1").
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(1, 3))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout-all", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", w.Code)
	}
}
