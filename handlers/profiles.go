package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/middleware"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/profiles] - retrieves all profiles for the logged-in user.
func GetProfiles(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var profiles []models.Profile
		dbErr := database.Select(&profiles, "SELECT * FROM profiles WHERE user_id=$1 ORDER BY name ASC", userId)
		if dbErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve profiles"})
			return
		}

		if profiles == nil {
			profiles = []models.Profile{} // Return empty list instead of null
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Profiles retrieved successfully",
			"data":    profiles,
		})
	}
}

// GET [/api/v1/profiles/:profileId] - retrieves a single profile by ID for the logged-in user.
func GetProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		profileID := c.Param("profileId")
		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile ID is required"})
			return
		}
		var profile models.Profile
		dbErr := database.Get(&profile, "SELECT * FROM profiles WHERE id=$1 AND user_id=$2", profileID, userId)
		if dbErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Profile retrieved successfully",
			"data":    profile,
		})
	}
}

// POST [/api/v1/profiles] - creates a new profile for the logged-in user.
func CreateProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		var input struct {
			Name        string  `json:"name" binding:"required"`
			Description *string `json:"description"`
			Icon        *string `json:"icon"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		var newProfile models.Profile
		query := `
			INSERT INTO profiles (user_id, name, description, icon, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			RETURNING id, user_id, name, description, icon, created_at, updated_at`

		dbErr := database.Get(&newProfile, query, userId, input.Name, input.Description, input.Icon)
		if dbErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create profile: " + dbErr.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Profile created successfully",
			"data":    newProfile,
		})
	}
}

// PATCH [/api/v1/profiles/:profileId] - updates an existing profile for the logged-in user.
func UpdateProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		profileID := c.Param("profileId")
		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile ID is required"})
			return
		}

		var input struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Icon        *string `json:"icon"`
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
				description = COALESCE($2, description),
				icon = COALESCE($3, icon),
				updated_at = NOW()
			WHERE id = $4 AND user_id = $5
			RETURNING id, user_id, name, description, icon, created_at, updated_at`

		dbErr := database.Get(&updatedProfile, query, input.Name, input.Description, input.Icon, profileID, userId)
		if dbErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile: " + dbErr.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Profile updated successfully",
			"data":    updatedProfile,
		})
	}
}

// DELETE [/api/v1/profiles/:profileId] - deletes a profile for the logged-in user.
func DeleteProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "data": false})
			return
		}
		profileID := c.Param("profileId")
		if profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile ID is required", "data": false})
			return
		}
		result, dbErr := database.Exec("DELETE FROM profiles WHERE id=$1 AND user_id=$2", profileID, userId)
		if dbErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete profile: " + dbErr.Error(), "data": false})
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found", "data": false})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Profile deleted successfully", "data": true})
	}
}
