package e2e

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

func setupE2EApp(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	gin.SetMode(gin.TestMode)
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	database := db.NewFromDB(sqlxDB)

	r := gin.New()
	router.SetupRoutes(r, database)

	return r, mock
}

func TestE2E_HealthCheck(t *testing.T) {
	r, _ := setupE2EApp(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "UP" {
		t.Errorf("expected UP status, got %v", resp["status"])
	}
}

func TestE2E_KubernetesProbes(t *testing.T) {
	r, _ := setupE2EApp(t)

	// Liveness Probe
	wLive := httptest.NewRecorder()
	reqLive, _ := http.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	r.ServeHTTP(wLive, reqLive)
	if wLive.Code != http.StatusOK {
		t.Errorf("expected status 200 for liveness probe, got %d", wLive.Code)
	}

	// Readiness Probe
	wReady := httptest.NewRecorder()
	reqReady, _ := http.NewRequest(http.MethodGet, "/api/v1/health/ready", nil)
	r.ServeHTTP(wReady, reqReady)
	if wReady.Code != http.StatusOK {
		t.Errorf("expected status 200 for readiness probe, got %d", wReady.Code)
	}

	// Startup Probe
	wStartup := httptest.NewRecorder()
	reqStartup, _ := http.NewRequest(http.MethodGet, "/api/v1/health/startup", nil)
	r.ServeHTTP(wStartup, reqStartup)
	if wStartup.Code != http.StatusOK {
		t.Errorf("expected status 200 for startup probe, got %d", wStartup.Code)
	}
}

func TestE2E_FullUserSignupAndLoginJourney(t *testing.T) {
	os.Setenv("JWT_SECRET", "e2e-secret-key-1234567890123456")
	defer os.Unsetenv("JWT_SECRET")

	r, mock := setupE2EApp(t)
	userID := uuid.New()
	email := "e2e_user@example.com"
	rawAuthKey := "client-derived-auth-key"

	// 1. Signup
	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnError(sqlmock.ErrCancelled)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs(email, "E2E User", "https://avatar.url", sqlmock.AnyArg(), "auth-salt", "enc-salt").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "avatar_url", "created_at", "updated_at"}).
			AddRow(userID, email, "E2E User", "https://avatar.url", time.Now(), time.Now()))

	signupPayload, _ := json.Marshal(map[string]string{
		"email":          email,
		"name":           "E2E User",
		"avatarUrl":      "https://avatar.url",
		"authKey":        rawAuthKey,
		"authSalt":       "auth-salt",
		"encryptionSalt": "enc-salt",
	})

	wSignup := httptest.NewRecorder()
	reqSignup, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(signupPayload))
	reqSignup.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wSignup, reqSignup)

	if wSignup.Code != http.StatusCreated {
		t.Fatalf("signup failed with status %d: %s", wSignup.Code, wSignup.Body.String())
	}

	// 2. Fetch Salt
	mock.ExpectQuery("SELECT auth_salt, encryption_salt, recovery_salt FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"auth_salt", "encryption_salt", "recovery_salt"}).
			AddRow("auth-salt", "enc-salt", nil))

	wSalt := httptest.NewRecorder()
	reqSalt, _ := http.NewRequest(http.MethodGet, "/api/v1/users/salt?email="+email, nil)
	r.ServeHTTP(wSalt, reqSalt)

	if wSalt.Code != http.StatusOK {
		t.Fatalf("salt fetch failed with status %d", wSalt.Code)
	}

	// 3. Login
	hashedKey, _ := bcrypt.GenerateFromPassword([]byte(rawAuthKey), bcrypt.DefaultCost)
	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "auth_key", "name", "created_at", "updated_at"}).
			AddRow(userID, email, string(hashedKey), "E2E User", time.Now(), time.Now()))

	loginPayload, _ := json.Marshal(map[string]string{
		"email":   email,
		"authKey": rawAuthKey,
	})

	wLogin := httptest.NewRecorder()
	reqLogin, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginPayload))
	reqLogin.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusCreated {
		t.Fatalf("login failed with status %d: %s", wLogin.Code, wLogin.Body.String())
	}

	var loginData struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginData)
	accessToken := loginData.Data.AccessToken
	if accessToken == "" {
		t.Fatalf("login did not return accessToken")
	}

	// 4. Access Protected Route (GET /api/v1/users/)
	mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "avatar_url", "created_at", "updated_at"}).
			AddRow(userID, email, "E2E User", "https://avatar.url", time.Now(), time.Now()))

	wProfile := httptest.NewRecorder()
	reqProfile, _ := http.NewRequest(http.MethodGet, "/api/v1/users/", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+accessToken)
	r.ServeHTTP(wProfile, reqProfile)

	if wProfile.Code != http.StatusOK {
		t.Fatalf("profile fetch failed with status %d", wProfile.Code)
	}
}

func TestE2E_FullAccountRecoveryJourney(t *testing.T) {
	os.Setenv("JWT_SECRET", "e2e-recovery-secret-key-12345678")
	defer os.Unsetenv("JWT_SECRET")

	r, mock := setupE2EApp(t)
	userID := uuid.New()
	email := "recover_user@example.com"
	rawRecoveryHash := "argon2-client-recovery-hash"
	newAuthKey := "new-derived-auth-key"

	// 1. Setup recovery
	mock.ExpectQuery("SELECT id, email FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(userID, email))

	mock.ExpectExec("UPDATE users").
		WithArgs(sqlmock.AnyArg(), "recovery-salt-xyz", userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	setupBody, _ := json.Marshal(map[string]string{
		"email":        email,
		"recoverySalt": "recovery-salt-xyz",
		"recoveryHash": rawRecoveryHash,
	})

	wSetup := httptest.NewRecorder()
	reqSetup, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/recovery/setup", bytes.NewReader(setupBody))
	reqSetup.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wSetup, reqSetup)

	if wSetup.Code != http.StatusOK {
		t.Fatalf("recovery setup failed: %s", wSetup.Body.String())
	}

	// 2. Initiate recovery (Forgot Password)
	bcryptRecoveryHash, _ := bcrypt.GenerateFromPassword([]byte(rawRecoveryHash), bcrypt.DefaultCost)
	mock.ExpectQuery("SELECT id, email, recovery_hash FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "recovery_hash"}).
			AddRow(userID, email, string(bcryptRecoveryHash)))

	initiateBody, _ := json.Marshal(map[string]string{
		"email":        email,
		"recoveryHash": rawRecoveryHash,
	})

	wInit := httptest.NewRecorder()
	reqInit, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/recovery/initiate", bytes.NewReader(initiateBody))
	reqInit.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wInit, reqInit)

	if wInit.Code != http.StatusOK {
		t.Fatalf("recovery initiate failed: %s", wInit.Body.String())
	}

	var initResp struct {
		Data struct {
			RestoreJWT string `json:"restoreJwt"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wInit.Body.Bytes(), &initResp)
	restoreJWT := initResp.Data.RestoreJWT

	// 3. Confirm reset password & fetch salts
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
		t.Fatalf("reset password confirm failed: %s", wConfirm.Body.String())
	}

	// 4. Complete recovery with new credentials
	mock.ExpectQuery("SELECT (.+) FROM users WHERE id = \\$1").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(userID, email))

	mock.ExpectExec("UPDATE users").
		WithArgs(sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	completeBody, _ := json.Marshal(map[string]string{
		"restoreJwt": restoreJWT,
		"authKey":    newAuthKey,
	})

	wComplete := httptest.NewRecorder()
	reqComplete, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/recovery/complete", bytes.NewReader(completeBody))
	reqComplete.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wComplete, reqComplete)

	if wComplete.Code != http.StatusOK {
		t.Fatalf("recovery complete failed: %s", wComplete.Body.String())
	}

	// 5. Login with new credentials
	newHashedAuthKey, _ := bcrypt.GenerateFromPassword([]byte(newAuthKey), bcrypt.DefaultCost)
	mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "auth_key", "name", "created_at", "updated_at"}).
			AddRow(userID, email, string(newHashedAuthKey), "Recover User", time.Now(), time.Now()))

	loginBody, _ := json.Marshal(map[string]string{
		"email":   email,
		"authKey": newAuthKey,
	})

	wLogin := httptest.NewRecorder()
	reqLogin, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusCreated {
		t.Fatalf("login with new credentials failed: %s", wLogin.Body.String())
	}
}

func TestE2E_VaultEntryLifecycleJourney(t *testing.T) {
	os.Setenv("JWT_SECRET", "e2e-vault-secret-key-1234567890")
	defer os.Unsetenv("JWT_SECRET")

	r, mock := setupE2EApp(t)
	userID := uuid.New()
	profileID := uuid.New()
	entryID := uuid.New()

	// Generate bearer token for authenticated requests
	token, _ := middlewareGenerateTestToken(userID, "e2e-vault-secret-key-1234567890")

	// 1. Create Profile
	mock.ExpectQuery("INSERT INTO profiles").
		WithArgs(userID, "Work Profile", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "is_archived", "created_at", "updated_at"}).
			AddRow(profileID, userID, "Work Profile", false, time.Now(), time.Now()))

	profileBody, _ := json.Marshal(map[string]string{
		"name": "Work Profile",
	})
	wProfile := httptest.NewRecorder()
	reqProfile, _ := http.NewRequest(http.MethodPost, "/api/v1/profiles", bytes.NewReader(profileBody))
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	reqProfile.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wProfile, reqProfile)

	if wProfile.Code != http.StatusCreated {
		t.Fatalf("create profile failed: %s", wProfile.Body.String())
	}

	// 2. Create Entry
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM profiles WHERE id = \\$1 AND user_id = \\$2").
		WithArgs(profileID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("INSERT INTO vault_entries").
		WithArgs(profileID, sqlmock.AnyArg(), "login", "GitHub Login", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "user@github.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), false, false).
		WillReturnRows(sqlmock.NewRows([]string{"id", "profile_id", "type", "name", "created_at", "updated_at"}).
			AddRow(entryID, profileID, "login", "GitHub Login", time.Now(), time.Now()))

	entryBody, _ := json.Marshal(map[string]interface{}{
		"type":          "login",
		"name":          "GitHub Login",
		"email":         "user@github.com",
		"encryptedBlob": "U29tZSBlbmNyeXB0ZWQgZGF0YQ==",
	})
	wEntry := httptest.NewRecorder()
	reqEntry, _ := http.NewRequest(http.MethodPost, "/api/v1/entries", bytes.NewReader(entryBody))
	reqEntry.Header.Set("Authorization", "Bearer "+token)
	reqEntry.Header.Set("X-Profile-Id", profileID.String())
	reqEntry.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wEntry, reqEntry)

	if wEntry.Code != http.StatusCreated {
		t.Fatalf("create entry failed: %s", wEntry.Body.String())
	}

	// 3. Delete Entry
	mock.ExpectExec("DELETE FROM vault_entries e USING profiles p WHERE e.id = \\$1 AND e.profile_id = p.id AND p.user_id = \\$2").
		WithArgs(entryID.String(), userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	wDel := httptest.NewRecorder()
	reqDel, _ := http.NewRequest(http.MethodDelete, "/api/v1/entries/"+entryID.String(), nil)
	reqDel.Header.Set("Authorization", "Bearer "+token)
	reqDel.Header.Set("X-Profile-Id", profileID.String())
	r.ServeHTTP(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("delete entry failed: %s", wDel.Body.String())
	}
}

func middlewareGenerateTestToken(userID uuid.UUID, secret string) (string, error) {
	return handlers.GenerateJWT(userID, 1*time.Hour)
}
