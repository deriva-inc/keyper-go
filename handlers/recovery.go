package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// RestoreJWTClaims represents the JWT claims for account recovery.
type RestoreJWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Type   string    `json:"type"` // always "restore"
	jwt.RegisteredClaims
}

// GenerateRestoreJWT creates a temporary token for the recovery flow.
func GenerateRestoreJWT(userID uuid.UUID, email string, expiryTime time.Duration) (string, error) {
	expirationTime := time.Now().Add(expiryTime)

	claims := &RestoreJWTClaims{
		UserID: userID,
		Email:  email,
		Type:   "restore",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "keyper-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}

	return token.SignedString([]byte(jwtSecret))
}

// ParseAndValidateRestoreJWT parses and verifies a restore JWT.
func ParseAndValidateRestoreJWT(tokenString string) (*RestoreJWTClaims, error) {
	claims := &RestoreJWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			return nil, fmt.Errorf("JWT_SECRET environment variable not set")
		}

		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid || claims.Type != "restore" {
		return nil, fmt.Errorf("invalid restore token")
	}

	return claims, nil
}

// extractRestoreToken reads the restore token from the request body or Bearer Authorization header.
func extractRestoreToken(c *gin.Context, bodyToken string) (string, error) {
	if strings.TrimSpace(bodyToken) != "" {
		return strings.TrimSpace(bodyToken), nil
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("restore token is required in request payload (restoreJwt) or Authorization header")
}

// POST [/api/v1/auth/recovery/setup]
// Sets up recovery credentials (recovery_hash and recovery_salt) for a user.
type RecoverySetupReq struct {
	Email        *string `json:"email"`
	RecoveryHash *string `json:"recoveryHash"`
	RecoverySalt string  `json:"recoverySalt" binding:"required"`
}

func RecoverySetup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RecoverySetupReq
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
			return
		}

		if input.Email == nil || *input.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email must be provided"})
			return
		}

		var rawHash string
		if input.RecoveryHash != nil && *input.RecoveryHash != "" {
			rawHash = *input.RecoveryHash
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recoveryHash is required"})
			return
		}

		var existingUser models.User
		var getUserErr error
		getUserErr = database.Get(&existingUser, "SELECT id, email FROM users WHERE email = $1", *input.Email)

		if getUserErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Hash recovery_hash with pepper for storage.
		pepper := GetPepper()
		hashedRecoveryHash, err := GeneratePepperedHash(rawHash, pepper)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure recovery credentials"})
			return
		}

		updateQuery := `
			UPDATE users
			SET recovery_hash = $1,
			    recovery_salt = $2,
			    updated_at = NOW()
			WHERE id = $3`

		_, err = database.Exec(updateQuery, string(hashedRecoveryHash), input.RecoverySalt, existingUser.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update recovery settings: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Recovery setup successfully",
			"data": gin.H{
				"userId": existingUser.ID,
				"email":  existingUser.Email,
			},
		})
	}
}

// POST [/api/v1/auth/recovery/initiate]
// Initiates the account recovery process and returns a restore_jwt upon successful verification.
type RecoveryInitiateReq struct {
	Email        string  `json:"email" binding:"required,email"`
	RecoveryHash *string `json:"recoveryHash"`
}

func RecoveryInitiate(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RecoveryInitiateReq
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
			return
		}

		var clientHash string
		if input.RecoveryHash != nil && *input.RecoveryHash != "" {
			clientHash = *input.RecoveryHash
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recoveryHash is required"})
			return
		}

		var user struct {
			ID           uuid.UUID `db:"id"`
			Email        string    `db:"email"`
			RecoveryHash *string   `db:"recovery_hash"`
		}

		query := "SELECT id, email, recovery_hash FROM users WHERE email = $1"
		if err := database.Get(&user, query, input.Email); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		if user.RecoveryHash == nil || *user.RecoveryHash == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Recovery credentials not set for this account"})
			return
		}

		pepper := GetPepper()
		matched, _ := ComparePasswordWithPepper(*user.RecoveryHash, clientHash, pepper)
		if !matched {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid recovery hash"})
			return
		}

		// Issue restore_jwt with 15-minute validity
		restoreJWT, err := GenerateRestoreJWT(user.ID, user.Email, 15*time.Minute)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate restore token: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Recovery initiated successfully",
			"data": gin.H{
				"restoreJwt": restoreJWT,
				"expiresIn":  900,
			},
		})
	}
}

// POST [/api/v1/auth/reset-password/confirm]
// Validates restore_jwt and supplies current salts and next steps to the client.
type ResetPasswordConfirmReq struct {
	RestoreJWT string `json:"restoreJwt"`
}

func ResetPasswordConfirm(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ResetPasswordConfirmReq
		_ = c.ShouldBindJSON(&input)

		tokenStr, err := extractRestoreToken(c, input.RestoreJWT)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		claims, err := ParseAndValidateRestoreJWT(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired restore token: " + err.Error()})
			return
		}

		var user struct {
			ID             uuid.UUID `db:"id"`
			Email          string    `db:"email"`
			AuthSalt       string    `db:"auth_salt"`
			EncryptionSalt string    `db:"encryption_salt"`
		}
		if err := database.Get(&user, "SELECT id, email, auth_salt, encryption_salt FROM users WHERE id = $1", claims.UserID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Password reset confirmed",
			"data": gin.H{
				"userId":         user.ID,
				"email":          user.Email,
				"authSalt":       user.AuthSalt,
				"encryptionSalt": user.EncryptionSalt,
			},
		})
	}
}

// POST [/api/v1/auth/recovery/complete]
// Finalizes account recovery by storing the new auth key.
type RecoveryCompleteReq struct {
	RestoreJWT string `json:"restoreJwt"`
	AuthKey    string `json:"authKey" binding:"required"`
}

func RecoveryComplete(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input RecoveryCompleteReq
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
			return
		}

		tokenStr, err := extractRestoreToken(c, input.RestoreJWT)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		claims, err := ParseAndValidateRestoreJWT(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired restore token: " + err.Error()})
			return
		}

		var existingUser models.User
		if err := database.Get(&existingUser, "SELECT * FROM users WHERE id = $1", claims.UserID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Hash new auth_key with pepper
		pepper := GetPepper()
		hashedAuthKey, err := GeneratePepperedHash(input.AuthKey, pepper)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash new auth key"})
			return
		}

		updateQuery := `
			UPDATE users
			SET auth_key = $1,
			    updated_at = NOW()
			WHERE id = $2`

		_, err = database.Exec(updateQuery, string(hashedAuthKey), existingUser.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user credentials: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Recovery complete. Credentials updated successfully. Please log in with your new password.",
			"data": gin.H{
				"userId":  existingUser.ID,
				"email":   existingUser.Email,
				"reroute": "login",
			},
		})
	}
}
