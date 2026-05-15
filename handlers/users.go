package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/users/salt] - retrieves the salt for a given user's email.
func GetUserSalt(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Query("email")
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email query parameter is required"})
			return
		}

		var UserSalts struct {
			AuthSalt       string `db:"auth_salt"`
			EncryptionSalt string `db:"encryption_salt"`
		}

		err := database.Get(&UserSalts, "SELECT auth_salt, encryption_salt FROM users WHERE email = $1", email)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Salt retrieved successfully",
			"data": gin.H{
				"authSalt":       UserSalts.AuthSalt,
				"encryptionSalt": UserSalts.EncryptionSalt,
			},
		})
	}
}

// GET [/api/v1/users/me] - retrieves the details of the currently logged-in user.
func GetUserProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}

		var user models.User
		query := "SELECT id, email, name, avatar_url, created_at, updated_at FROM users WHERE id = $1"
		err := database.Get(&user, query, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, user)
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
			Email     *string `json:"email" binding:"omitempty,email"`
			Name      *string `json:"name"`
			AvatarURL *string `json:"avatarUrl"`
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
                name = COALESCE($2, name), 
                avatar_url = COALESCE($3, avatar_url),
                updated_at = NOW()
            WHERE id = $4
            RETURNING id, email, name, avatar_url, created_at, updated_at`

		err := database.Get(&updatedUser, query, input.Email, input.Name, input.AvatarURL, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedUser)
	}
}
