package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/deriva-inc/keyper-go/words"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type RegisterUserReq struct {
	Email          string `json:"email" binding:"required,email"`
	Name           string `json:"name" binding:"required"`
	AvatarURL      string `json:"avatarUrl" binding:"required,url"`
	AuthSalt       string `json:"authSalt" binding:"required"`
	AuthKey        string `json:"authKey" binding:"required"`
	EncryptionSalt string `json:"encryptionSalt" binding:"required"`
}

type LoginUserReq struct {
	Email      string      `json:"email" binding:"required,email"`
	AuthKey    string      `json:"authKey" binding:"required"`
	RememberMe bool        `json:"rememberMe"`
	DeviceInfo *DeviceInfo `json:"deviceInfo,omitempty"`
}

type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

// Defines the structure of our recovery key: 4 words and 1 number.
const (
	recoveryKeyWordCount = 4
	recoveryKeyNumberMax = 99
)

// GenerateRecoveryKey creates a human-readable, high-entropy recovery key.
// Example: "keyper-recovery-apple-rocket-73-winter"
func GenerateRecoveryKey() (string, error) {
	var keyParts []string
	keyParts = append(keyParts, "keyper-recovery")

	wordListLen := big.NewInt(int64(len(words.WordList)))

	// Add random words
	for i := 0; i < recoveryKeyWordCount; i++ {
		randomIndex, err := rand.Int(rand.Reader, wordListLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random word index: %w", err)
		}
		keyParts = append(keyParts, words.WordList[randomIndex.Int64()])
	}

	// Add a random number for extra entropy
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(recoveryKeyNumberMax+1))
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	// Insert the number in a semi-random position to make it less predictable.
	// We'll insert it before the last word.
	numStr := fmt.Sprintf("%02d", randomNumber.Int64()) // Pad with zero e.g., 07
	lastWord := keyParts[len(keyParts)-1]
	keyParts[len(keyParts)-1] = numStr
	keyParts = append(keyParts, lastWord)

	return strings.Join(keyParts, "-"), nil
}

// GenerateJWT creates a new signed JWT for a given user ID.
// The JWT secret should be stored securely in environment variables.
func GenerateJWT(userID uuid.UUID, expiryTime time.Duration) (string, error) {
	expirationTime := time.Now().Add(expiryTime)

	// Create the JWT claim.
	claims := &JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "keyper-api",
		},
	}

	// Create a new token object, specifying the signing method and the claims.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with your secret key to get the complete, signed token string.
	// The JWT_SECRET must be loaded from your environment for security.
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}

	return token.SignedString([]byte(jwtSecret))
}

// POST [/api/v1/auth/signup] - registers a new user and creates their profile.
func RegisterUser(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate the incoming request body.
		var input RegisterUserReq
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		getUserQuery := "SELECT * FROM users WHERE email = $1"
		var existingUser = models.User{}
		getUserFromDBErr := database.Get(&existingUser, getUserQuery, input.Email)
		if getUserFromDBErr == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}

		// Hashing the AuthKey once again on the server with pepper for defense-in-depth.
		clientAuthKey := input.AuthKey
		pepper := GetPepper()

		hashedAuthKeyForDB, err := GeneratePepperedHash(clientAuthKey, pepper)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure user credentials"})
			return
		}

		// Generate a recovery key to return to the client for recovery setup.
		recoveryKey, err := GenerateRecoveryKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate recovery key"})
			return
		}

		// Call our business logic to create the user.
		var newUser models.User

		query := `
            INSERT INTO users (email, name, avatar_url, auth_key, auth_salt, encryption_salt, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
            RETURNING id, email, name, avatar_url, created_at, updated_at`

		createUserInDBErr := database.Get(&newUser, query, input.Email, input.Name, input.AvatarURL, string(hashedAuthKeyForDB), input.AuthSalt, input.EncryptionSalt)
		if createUserInDBErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + createUserInDBErr.Error()})
			return
		}

		// Return a success response.
		// Note: We are NOT sending any hashes back, only safe information.
		c.JSON(http.StatusCreated, gin.H{
			"message": "User registered successfully",
			"data": gin.H{
				"user": gin.H{
					"id":          newUser.ID,
					"email":       newUser.Email,
					"createdAt":   newUser.CreatedAt,
					"updatedAt":   newUser.UpdatedAt,
					"name":        newUser.Name,
					"avatarUrl":   newUser.AvatarURL,
					"recoveryKey": recoveryKey, // This is the only time we send the recovery key to the client
				},
			},
		})
	}
}

func Login(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate the incoming request body.
		var input LoginUserReq
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		getUserQuery := "SELECT * FROM users WHERE email = $1"
		var existingUser = models.User{}
		getUserFromDBErr := database.Get(&existingUser, getUserQuery, input.Email)
		if getUserFromDBErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Compare hashes with pepper, seamlessly supporting legacy unpeppered hashes with auto-upgrade.
		pepper := GetPepper()
		matched, needsRehash := ComparePasswordWithPepper(existingUser.AuthKey, input.AuthKey, pepper)
		if !matched {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Opportunistically upgrade legacy unpeppered records upon successful login
		if needsRehash && pepper != "" {
			if upgradedHash, err := GeneratePepperedHash(input.AuthKey, pepper); err == nil {
				_, _ = database.Exec("UPDATE users SET auth_key = $1, updated_at = NOW() WHERE id = $2", upgradedHash, existingUser.ID)
			}
		}

		// Generate JWT for the session (access token always 24h).
		accessToken, jwtErr := GenerateJWT(existingUser.ID, 24*time.Hour)
		if jwtErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token: " + jwtErr.Error()})
			return
		}

		// Handle session & refresh token if rememberMe is requested
		var refreshToken string
		var sessionExpiry *time.Time

		if input.RememberMe {
			rt, err := GenerateRefreshToken()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token: " + err.Error()})
				return
			}
			refreshToken = rt

			// Session duration is 7 days when rememberMe is true
			expiry := time.Now().Add(7 * 24 * time.Hour)
			sessionExpiry = &expiry

			// Determine device info: use provided deviceInfo or parse User-Agent
			devInfo := ParseUserAgent(c.GetHeader("User-Agent"))
			if input.DeviceInfo != nil {
				if input.DeviceInfo.Browser != nil && *input.DeviceInfo.Browser != "" {
					devInfo.Browser = input.DeviceInfo.Browser
				}
				if input.DeviceInfo.OS != nil && *input.DeviceInfo.OS != "" {
					devInfo.OS = input.DeviceInfo.OS
				}
				if input.DeviceInfo.Manufacturer != nil && *input.DeviceInfo.Manufacturer != "" {
					devInfo.Manufacturer = input.DeviceInfo.Manufacturer
				}
			}

			insertSessionQuery := `
				INSERT INTO user_sessions (user_id, refresh_token, expiry_time, browser, os, manufacturer, creation_time, last_active)
				VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`
			hashedRefreshToken := HashRefreshToken(refreshToken)
			_, err = database.Exec(insertSessionQuery, existingUser.ID, hashedRefreshToken, expiry, devInfo.Browser, devInfo.OS, devInfo.Manufacturer)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user session: " + err.Error()})
				return
			}
		}

		userResponse := gin.H{
			"id":        existingUser.ID,
			"email":     existingUser.Email,
			"name":      existingUser.Name,
			"avatarUrl": existingUser.AvatarURL,
			"createdAt": existingUser.CreatedAt,
			"updatedAt": existingUser.UpdatedAt,
		}

		responseData := gin.H{
			"accessToken": accessToken,
			"user":        userResponse,
		}
		if refreshToken != "" {
			responseData["refreshToken"] = refreshToken
			responseData["expiryTime"] = sessionExpiry
		}

		// return success reponse
		c.JSON(http.StatusCreated, gin.H{
			"message": "Login successful",
			"data":    responseData,
		})
	}
}

func GetMe(c *gin.Context) {}
