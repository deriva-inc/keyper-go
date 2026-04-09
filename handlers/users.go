package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/users/me] - retrieves the details of the currently logged-in user.
func GetUserProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("x-user-id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}

		var user models.User
		query := "SELECT id, email, display_name, avatar_url, created_at, updated_at FROM users WHERE id = $1"
		err := database.Get(&user, query, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// POST [/api/v1/users] - creates a new user profile.
func PostUserProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Email        string  `json:"email" binding:"required,email"`
			DisplayName  *string `json:"displayName"`
			AvatarURL    *string `json:"avatarUrl"`
			AuthHash     string  `json:"authHash"`
			Salt         string  `json:"salt"`
			RecoveryHash *string `json:"recoveryHash"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		var newUser models.User
		query := `
            INSERT INTO users (email, display_name, avatar_url, auth_hash, salt, recovery_hash, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
            RETURNING id, email, display_name, avatar_url, created_at, updated_at`

		err := database.Get(&newUser, query, input.Email, input.DisplayName, input.AvatarURL, input.AuthHash, input.Salt, input.RecoveryHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, newUser)
	}
}

// PATCH [/api/v1/users/:id] - updates user profile details.
func UpdateUserProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		var input struct {
			Email       *string `json:"email" binding:"omitempty,email"`
			DisplayName *string `json:"displayName"`
			AvatarURL   *string `json:"avatarUrl"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		var updatedUser models.User
		query := `
            UPDATE users 
            SET 
                email = COALESCE($1, email), 
                display_name = COALESCE($2, display_name), 
                avatar_url = COALESCE($3, avatar_url),
                updated_at = NOW()
            WHERE id = $4
            RETURNING id, email, display_name, avatar_url, created_at, updated_at`

		err := database.Get(&updatedUser, query, input.Email, input.DisplayName, input.AvatarURL, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedUser)
	}
}
