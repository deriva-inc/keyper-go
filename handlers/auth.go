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
	"golang.org/x/crypto/bcrypt"
)

type RegisterUserReq struct {
	Email     string `json:"email" binding:"required,email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	AuthHash  string `json:"authHash"`
	Salt      string `json:"salt"`
}

type LoginUserReq struct {
	Email    string `json:"email" binding:"required,email"`
	AuthHash string `json:"authHash"`
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

		// Hashing the AuthHash once again on the server for more security.
		clientAuthHash := input.AuthHash

		hashedAuthHashForDB, err := bcrypt.GenerateFromPassword([]byte(clientAuthHash), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure user credentials"})
			return
		}

		// Generate a recovery key and hash it for storage.
		recoveryKey, err := GenerateRecoveryKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate recovery key"})
			return
		}

		// Generate the bcrypt hash of the recovery key to be stored in the database.
		recoveryHash, recHashErr := bcrypt.GenerateFromPassword([]byte(recoveryKey), bcrypt.DefaultCost)
		if recHashErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash recovery key"})
			return
		}

		// Call our business logic to create the user.
		var newUser models.User

		query := `
            INSERT INTO users (email, name, avatar_url, auth_hash, salt, recovery_hash, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
            RETURNING id, email, name, avatar_url, created_at, updated_at`

		createUserInDBErr := database.Get(&newUser, query, input.Email, input.Name, input.AvatarURL, string(hashedAuthHashForDB), input.Salt, recoveryHash)
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

		// Compare hashes, one sent by the client and one stored in the database.
		err := bcrypt.CompareHashAndPassword([]byte(existingUser.AuthHash), []byte(input.AuthHash))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate JWT for the session.
		accessToken, jwtErr := GenerateJWT(existingUser.ID, 60*time.Minute)
		if jwtErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token: " + jwtErr.Error()})
			return
		}

		userResponse := gin.H{
			"id":        existingUser.ID,
			"email":     existingUser.Email,
			"name":      existingUser.Name,
			"avatarUrl": existingUser.AvatarURL,
			"createdAt": existingUser.CreatedAt,
			"updatedAt": existingUser.UpdatedAt,
		}

		// return success reponse
		c.JSON(http.StatusCreated, gin.H{
			"message": "Login successful",
			"data": gin.H{
				"accessToken": accessToken,
				"user":        userResponse,
			},
		})
	}
}

func GetMe(c *gin.Context) {}
