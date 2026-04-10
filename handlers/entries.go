package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VaultEntryInput defines the structure for creating/updating a vault entry.
type VaultEntryInput struct {
	GroupID       *uuid.UUID      `json:"groupId"`
	Type          string          `json:"type" binding:"required"`
	Name          string          `json:"name" binding:"required"`
	EncryptedBlob string          `json:"encryptedBlob" binding:"required"` // Received as Base64 string from frontend
	CustomFields  json.RawMessage `json:"customFields"`                     // Handle as raw JSON
	IsFavorite    bool            `json:"isFavorite"`
}

// POST [/api/v1/entries] - saves a new vault entry to the database.
func CreateEntry(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.GetHeader("X-Profile-Id")

		if userID == "" || profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id / X-Profile-Id headers are required"})
			return
		}

		var input VaultEntryInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
			return
		}

		// Security check: ensure profile belongs to user
		var count int
		err := database.Get(&count, "SELECT COUNT(*) FROM profiles WHERE id = $1 AND user_id = $2", profileID, userID)
		if err != nil || count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid profile or access denied"})
			return
		}

		// Decode the base64-encoded encrypted blob before storing in DB
		encryptedBytes, err := base64.StdEncoding.DecodeString(input.EncryptedBlob)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 encoding for encryptedBlob"})
			return
		}

		var newEntry models.VaultEntry
		query := `
			INSERT INTO vault_entries (profile_id, group_id, type, name, encrypted_blob, custom_fields, is_favorite, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			RETURNING *`

		err = database.Get(&newEntry, query, profileID, input.GroupID, input.Type, input.Name, encryptedBytes, input.CustomFields, input.IsFavorite)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vault entry: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, newEntry)
	}
}

// GET [/api/v1/entries] - retrieves all entries for a specific profile.
func GetEntries(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		profileID := c.GetHeader("X-Profile-Id")

		if userID == "" || profileID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-User-Id and X-Profile-Id headers are required"})
			return
		}

		var entries []models.VaultEntry
		query := `
			SELECT e.* FROM vault_entries e
			JOIN profiles p ON e.profile_id = p.id
			WHERE e.profile_id = $1 AND p.user_id = $2
			ORDER BY e.name ASC`

		err := database.Select(&entries, query, profileID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve entries"})
			return
		}

		if entries == nil {
			entries = []models.VaultEntry{}
		}
		c.JSON(http.StatusOK, entries)
	}
}

// GET [/api/v1/entries/:entryId] - retrieves a single vault entry.
func GetEntry(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		entryID := c.Param("entryId")

		var entry models.VaultEntry
		query := `
			SELECT e.* FROM vault_entries e
			JOIN profiles p ON e.profile_id = p.id
			WHERE e.id = $1 AND p.user_id = $2`

		err := database.Get(&entry, query, entryID, userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found"})
			return
		}

		c.JSON(http.StatusOK, entry)
	}
}

// PATCH [/api/v1/entries/:entryId] - updates an existing vault entry.
func UpdateEntry(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		entryID := c.Param("entryId")

		var input struct {
			GroupID       *uuid.UUID       `json:"groupId"`
			Type          *string          `json:"type"`
			Name          *string          `json:"name"`
			EncryptedBlob *string          `json:"encryptedBlob"`
			CustomFields  *json.RawMessage `json:"customFields"`
			IsFavorite    *bool            `json:"isFavorite"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		var encryptedBytes []byte
		if input.EncryptedBlob != nil {
			var err error
			encryptedBytes, err = base64.StdEncoding.DecodeString(*input.EncryptedBlob)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 encoding for encryptedBlob"})
				return
			}
		}

		var updatedEntry models.VaultEntry
		query := `
			UPDATE vault_entries e
			SET 
				group_id = COALESCE($1, e.group_id),
				type = COALESCE($2, e.type::text)::entry_type,
				name = COALESCE($3, e.name),
				encrypted_blob = COALESCE($4, e.encrypted_blob),
				custom_fields = COALESCE($5, e.custom_fields),
				is_favorite = COALESCE($6, e.is_favorite),
				updated_at = NOW()
			FROM profiles p
			WHERE e.id = $7 AND e.profile_id = p.id AND p.user_id = $8
			RETURNING e.*`

		err := database.Get(&updatedEntry, query, input.GroupID, input.Type, input.Name, encryptedBytes, input.CustomFields, input.IsFavorite, entryID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update entry: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedEntry)
	}
}

// DELETE [/api/v1/entries/:entryId] - removes a vault entry.
func DeleteEntry(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-Id")
		entryID := c.Param("entryId")

		query := `
			DELETE FROM vault_entries e
			USING profiles p
			WHERE e.id = $1 AND e.profile_id = p.id AND p.user_id = $2`

		result, err := database.Exec(query, entryID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete entry"})
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found or access denied"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Entry deleted successfully"})
	}
}
