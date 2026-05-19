package handlers

import (
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/middleware"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
)

// GET [/api/v1/groups] - retrieves all groups within a specific profile for the logged-in user.
func GetAllGroupsInProfile(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		profileId := c.Query("profileId")

		var groups []models.Group
		query := `
			SELECT g.* FROM groups g
			JOIN profiles p ON g.profile_id = p.id
			WHERE g.profile_id = $1 AND p.user_id = $2
			ORDER BY g.name ASC`

		groupsErr := database.Select(&groups, query, profileId, userId)
		if groupsErr != nil {
			print("Error retrieving groups: ", groupsErr.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve groups"})
			return
		}

		if groups == nil {
			groups = []models.Group{}
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Groups retrieved successfully",
			"data":    groups,
		})
	}
}

// GET [/api/v1/groups/:groupId] - retrieves a single group's details if it belongs to the user's profile.
func GetGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		groupId := c.Param("groupId")
		// profileId := c.Query("profileId")

		var group models.Group
		query := `
			SELECT g.* FROM groups g
			JOIN profiles p ON g.profile_id = p.id
			WHERE g.id = $1 AND p.user_id = $2`

		groupErr := database.Get(&group, query, groupId, userId)
		if groupErr != nil {
			print("Error retrieving group: ", groupErr.Error())
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Group retrieved successfully",
			"data":    group,
		})
	}
}

// POST [/api/v1/groups] - creates a new group under a specific profile for the user.
func CreateGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		var CreateGroupInput struct {
			Name        string  `json:"name" binding:"required"`
			Description *string `json:"description"`
			Type        string  `json:"type" binding:"required,oneof=provider category"`
			ProfileId   string  `json:"profileId" binding:"required"`
			Icon        *string `json:"icon"`
		}

		if jsonBindErr := c.ShouldBindJSON(&CreateGroupInput); jsonBindErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		// Verify profile belongs to user
		var count int
		getProfileErr := database.Get(&count, "SELECT COUNT(*) FROM profiles WHERE id = $1 AND user_id = $2", CreateGroupInput.ProfileId, userId)
		if getProfileErr != nil || count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid profile or access denied"})
			return
		}

		var newGroup models.Group
		query := `
			INSERT INTO groups (profile_id, name, description, type, icon, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			RETURNING *`

		createNewGroupErr := database.Get(&newGroup, query, CreateGroupInput.ProfileId, CreateGroupInput.Name, CreateGroupInput.Description, CreateGroupInput.Type, CreateGroupInput.Icon)
		if createNewGroupErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Group created successfully",
			"data":    newGroup,
		})
	}
}

// PATCH [/api/v1/groups/:groupId] - updates an existing group's details.
func UpdateGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "data": false})
			return
		}

		groupID := c.Param("groupId")

		var input struct {
			Name *string `json:"name"`
			Icon *string `json:"icon"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
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

		updateGroupErr := database.Get(&updatedGroup, query, input.Name, input.Icon, groupID, userId)
		if updateGroupErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group: " + updateGroupErr.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Group updated successfully",
			"data":    updatedGroup,
		})
	}
}

// DELETE [/api/v1/groups/:groupId] - removes a group if it belongs to the user.
func DeleteGroup(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "data": false})
			return
		}
		groupID := c.Param("groupId")

		query := `
			DELETE FROM groups g
			USING profiles p
			WHERE g.id = $1 AND g.profile_id = p.id AND p.user_id = $2`

		result, err := database.Exec(query, groupID, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found or access denied"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Group deleted successfully", "data": true})
	}
}

// GET [/api/v1/groups/:groupId/entries/count] - retrieves the count of vault entries within a specific group.
func GetGroupEntryCount(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "data": false})
			return
		}

		groupId := c.Param("groupId")
		var count int
		query := `
			SELECT COUNT(*) from vault_entries ve
			JOIN groups g on ve.group_id = g.id
			JOIN profiles p on g.profile_id = p.id
			WHERE g.id = $1 AND p.user_id = $2`

		err = database.Get(&count, query, groupId, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entry count", "data": false})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Entry count retrieved successfully", "data": count})
	}
}
