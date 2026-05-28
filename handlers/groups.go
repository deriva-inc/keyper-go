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
			IsArchived  *bool   `json:"isArchived"`
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
			INSERT INTO groups (profile_id, name, description, type, icon, is_archived, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			RETURNING *`

		createNewGroupErr := database.Get(&newGroup, query, CreateGroupInput.ProfileId, CreateGroupInput.Name, CreateGroupInput.Description, CreateGroupInput.Type, CreateGroupInput.Icon, CreateGroupInput.IsArchived)
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
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Type        *string `json:"type" binding:"omitempty,oneof=provider category"`
			ProfileId   *string `json:"profileId"`
			Icon        *string `json:"icon"`
			IsArchived  *bool   `json:"isArchived"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		// Begin a transaction on the DB to maintain atomicity between the groups and vault_entries tables.
		tx, err := database.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction: " + err.Error()})
			return
		}
		// Ensure the transaction is always resolved — committed on success,
		// rolled back on any failure path below.
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				panic(p)
			}
		}()

		// Step 1: Update the group itself.
		var updatedGroup models.Group
		updateGroupQuery := `
			UPDATE groups g
			SET
				name        = COALESCE($1, g.name),
				description = COALESCE($2, g.description),
				type        = COALESCE($3, g.type),
				profile_id  = COALESCE($4, g.profile_id),
				icon        = COALESCE($5, g.icon),
				is_archived = COALESCE($6, g.is_archived),
				updated_at  = NOW()
			FROM profiles p
			WHERE g.id = $7
			AND g.profile_id = p.id
			AND p.user_id = $8
			RETURNING g.*`

		err = tx.Get(
			&updatedGroup, updateGroupQuery,
			input.Name,        // $1
			input.Description, // $2
			input.Type,        // $3
			input.ProfileId,   // $4
			input.Icon,        // $5
			input.IsArchived,  // $6
			groupID,           // $7
			userId,            // $8
		)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group: " + err.Error()})
			return
		}

		// Step 2: If the caller requested a profile_id change, cascade it
		// to every vault entry that belongs to this group.
		// We use the updatedGroup.ProfileID from the RETURNING clause above —
		// this is the authoritative new value regardless of what $4 was,
		// and it correctly handles the COALESCE (i.e. if ProfileId was nil,
		// updatedGroup.ProfileID is still the correct unchanged value so we
		// skip the update entirely).
		if input.ProfileId != nil {
			updateEntriesQuery := `
				UPDATE vault_entries
				SET
					profile_id = $1,
					updated_at = NOW()
				WHERE group_id = $2`

			_, err = tx.Exec(updateEntriesQuery, updatedGroup.ProfileID, groupID)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cascade profile update to entries: " + err.Error()})
				return
			}
		}

		// Both queries succeeded — commit the transaction.
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
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

// GET [/api/v1/groups/:groupId/entries] - retrieves all vault entries within a specific group.
func GetGroupEntries(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "data": false})
			return
		}

		groupId := c.Param("groupId")
		var entries []models.VaultEntry
		query := `
			SELECT ve.* from vault_entries ve
			JOIN groups g on ve.group_id = g.id
			JOIN profiles p on g.profile_id = p.id
			WHERE g.id = $1 AND p.user_id = $2`

		err = database.Select(&entries, query, groupId, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve entries", "data": false})
			return
		}

		if entries == nil {
			entries = []models.VaultEntry{}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Entries retrieved successfully", "data": entries})
	}
}

// GET [/api/v1/groups/:groupId/entries/count] - retrieves the count of vault entries within a specific group.
func GetGroupEntriesCount(database *db.DB) gin.HandlerFunc {
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
