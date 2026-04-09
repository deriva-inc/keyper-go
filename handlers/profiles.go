package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/profiles] - retrieves all profiles for the logged-in user.
func GetProfiles(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}

		var profiles []models.Profile
		err := database.Select(&profiles, "SELECT * FROM profiles WHERE user_id=$1 ORDER BY name ASC", userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve profiles"})
			return
		}

		if profiles == nil {
			profiles = []models.Profile{} // Return empty list instead of null
		}
		c.JSON(http.StatusOK, profiles)
	}
}

// GET [/api/v1/profiles/:profileId] - retrieves a single profile by ID for the logged-in user.
func GetProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.Param("profileId")

		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}
		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile ID is required"})
			return
		}

		var profile models.Profile
		err := database.Get(&profile, "SELECT * FROM profiles WHERE id=$1 AND user_id=$2", profileID, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
			return
		}

		c.JSON(http.StatusOK, profile)
	}
}

// POST [/api/v1/profiles] - creates a new profile for the logged-in user.
func CreateProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}

		var input struct {
			Name string  `json:"name" binding:"required"`
			Icon *string `json:"icon"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		var newProfile models.Profile
		query := `
			INSERT INTO profiles (user_id, name, icon, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			RETURNING id, user_id, name, icon, created_at, updated_at`

		err := database.Get(&newProfile, query, userID, input.Name, input.Icon)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create profile: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, newProfile)
	}
}

// PATCH [/api/v1/profiles/:profileId] - updates an existing profile for the logged-in user.
func UpdateProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.Param("profileId")

		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}
		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile ID is required"})
			return
		}

		var input struct {
			Name *string `json:"name"`
			Icon *string `json:"icon"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		var updatedProfile models.Profile
		query := `
			UPDATE profiles 
			SET 
				name = COALESCE($1, name), 
				icon = COALESCE($2, icon),
				updated_at = NOW()
			WHERE id = $3 AND user_id = $4
			RETURNING id, user_id, name, icon, created_at, updated_at`

		err := database.Get(&updatedProfile, query, input.Name, input.Icon, profileID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedProfile)
	}
}

// DELETE [/api/v1/profiles/:profileId] - deletes a profile for the logged-in user.
func DeleteProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.Param("profileId")

		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID missing from headers"})
			return
		}
		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile ID is required"})
			return
		}

		result, err := database.Exec("DELETE FROM profiles WHERE id=$1 AND user_id=$2", profileID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete profile: " + err.Error()})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Profile deleted successfully"})
	}
}
