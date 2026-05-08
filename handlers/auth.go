package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/deriva-inc/keyper-go/words"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type RegisterUserReq struct {
	Email     string `json:"email" binding:"required,email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	AuthHash  string `json:"authHash"`
	Salt      string `json:"salt"`
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

		// Call our business logic to create the user.
		var newUser models.User

		// Generate a recovery key and hash it for storage.
		recoveryKey, err := GenerateRecoveryKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate recovery key"})
			return
		}

		// Generate the bcrypt hash of the key to be stored in the database.
		recoveryHash, recHashErr := bcrypt.GenerateFromPassword([]byte(recoveryKey), bcrypt.DefaultCost)
		if recHashErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash recovery key"})
			return
		}

		// Return both to the client. The client will show the 'recoveryKey'
		// to the user and send the 'recoveryHash' to another endpoint to be stored.
		query := `
            INSERT INTO users (email, display_name, avatar_url, auth_hash, salt, recovery_hash, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
            RETURNING id, email, display_name, avatar_url, created_at, updated_at`

		createUserInDBErr := database.Get(&newUser, query, input.Email, input.Name, input.AvatarURL, input.AuthHash, input.Salt, recoveryHash)
		if createUserInDBErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + createUserInDBErr.Error()})
			return
		}

		// Return a success response.
		// Note: We are NOT sending the hash back, only safe information.
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

func Login(c *gin.Context) {
	// TODO: Implement user login logic
	c.JSON(200, "TODO: Login user")
}

func GetMe(c *gin.Context) {}
