package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/groups] - retrieves all groups within a specific profile for the logged-in user.
func GetAllGroupsInProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.GetHeader("X-Profile-Id")

		if userID == "" || profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id / X-Profile-Id headers are required"})
			return
		}

		var groups []models.Group
		query := `
			SELECT g.* FROM groups g
			JOIN profiles p ON g.profile_id = p.id
			WHERE g.profile_id = $1 AND p.user_id = $2
			ORDER BY g.name ASC`

		err := database.Select(&groups, query, profileID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve groups"})
			return
		}

		if groups == nil {
			groups = []models.Group{}
		}
		c.JSON(http.StatusOK, groups)
	}
}

// GET [/api/v1/groups/:groupId] - retrieves a single group's details if it belongs to the user's profile.
func GetGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		groupID := c.Param("groupId")

		var group models.Group
		query := `
			SELECT g.* FROM groups g
			JOIN profiles p ON g.profile_id = p.id
			WHERE g.id = $1 AND p.user_id = $2`

		err := database.Get(&group, query, groupID, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}

		c.JSON(http.StatusOK, group)
	}
}

// POST [/api/v1/groups] - creates a new group under a specific profile for the user.
func CreateGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.GetHeader("X-Profile-Id")

		if userID == "" || profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id / X-Profile-Id headers are required"})
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

		// Verify profile belongs to user
		var count int
		err := database.Get(&count, "SELECT COUNT(*) FROM profiles WHERE id = $1 AND user_id = $2", profileID, userID)
		if err != nil || count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid profile or access denied"})
			return
		}

		var newGroup models.Group
		query := `
			INSERT INTO groups (profile_id, name, icon, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			RETURNING *`

		err = database.Get(&newGroup, query, profileID, input.Name, input.Icon)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
			return
		}

		c.JSON(http.StatusCreated, newGroup)
	}
}

// PATCH [/api/v1/groups/:groupId] - updates an existing group's details.
func UpdateGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.GetHeader("X-Profile-Id")
		groupID := c.Param("groupId")

		if userID == "" || profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id / X-Profile-Id headers are required"})
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

		// Verify profile belongs to user
		var count int
		profileAccessErr := database.Get(&count, "SELECT COUNT(*) FROM profiles WHERE id = $1 AND user_id = $2", profileID, userID)
		if profileAccessErr != nil || count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid profile or access denied"})
			return
		}

		var updatedGroup models.Group
		query := `
			UPDATE groups g
			SET 
				name = COALESCE($1, g.name),
				icon = COALESCE($2, g.icon),
				updated_at = NOW()
			FROM profiles p
			WHERE g.id = $3 AND g.profile_id = p.id AND p.user_id = $4
			RETURNING g.*`

		err := database.Get(&updatedGroup, query, input.Name, input.Icon, groupID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedGroup)
	}
}

// DELETE [/api/v1/groups/:groupId] - removes a group if it belongs to the user.
func DeleteGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		groupID := c.Param("groupId")

		query := `
			DELETE FROM groups g
			USING profiles p
			WHERE g.id = $1 AND g.profile_id = p.id AND p.user_id = $2`

		result, err := database.Exec(query, groupID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found or access denied"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully"})
	}
}
