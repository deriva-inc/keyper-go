package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/deriva-inc/keyper-go/db"
	"github.com/deriva-inc/keyper-go/middleware"
	"github.com/deriva-inc/keyper-go/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VaultEntryInput defines the structure for creating/updating a vault entry.
type VaultEntryInput struct {
	Name           string          `json:"name" binding:"required"`
	Description    *string         `json:"description"`
	GroupID        string          `json:"groupId"`
	Type           string          `json:"type" binding:"required"`
	Icon           *string         `json:"icon"`
	EncryptedBlob  string          `json:"encryptedBlob" binding:"required"` // Received as Base64 string from frontend
	WebsiteURL     *string         `json:"websiteUrl,omitempty"`
	Email          *string         `json:"email" binding:"required"`
	UserID         *string         `json:"userId,omitempty"`
	UserName       *string         `json:"userName,omitempty"`
	CardNumber     *string         `json:"cardNumber,omitempty"`
	ExpirationDate *string         `json:"expirationDate,omitempty"`
	SecurityCode   *string         `json:"securityCode,omitempty"`
	CustomFields   json.RawMessage `json:"customFields"`
	IsFavorite     bool            `json:"isFavorite"`
	IsArchived     bool            `json:"isArchived"`
}

// POST [/api/v1/entries] - saves a new vault entry to the database.
func CreateEntry(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		profileId := c.GetHeader("X-Profile-Id")

		if profileId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Profile-Id headers are required"})
			return
		}

		var input VaultEntryInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
			return
		}

		// Security check: ensure profile belongs to user
		var count int
		profileCheckErr := database.Get(&count, "SELECT COUNT(*) FROM profiles WHERE id = $1 AND user_id = $2", profileId, userId)
		if profileCheckErr != nil || count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid profile or access denied"})
			return
		}

		// Decode the base64-encoded encrypted blob before storing in DB
		encryptedBytes, err := base64.StdEncoding.DecodeString(input.EncryptedBlob)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 encoding for encryptedBlob"})
			return
		}

		var customFields interface{}
		if len(input.CustomFields) > 0 && string(input.CustomFields) != "null" {
			customFields = input.CustomFields
		}

		var newEntry models.VaultEntry
		query := `
			INSERT INTO vault_entries (profile_id, group_id, type, name, description, icon, encrypted_blob, website_url, email, user_id, user_name, card_number, expiration_date, security_code, custom_fields, is_favorite, is_archived, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, NOW(), NOW())
			RETURNING *`

		// Convert empty string GroupID to nil for the database
		var groupID *uuid.UUID
		if input.GroupID != "" {
			parsedID, err := uuid.Parse(input.GroupID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid GroupID format"})
				return
			}
			groupID = &parsedID
		}

		err = database.Get(&newEntry, query, profileId, groupID, input.Type, input.Name, input.Description, input.Icon, encryptedBytes, input.WebsiteURL, input.Email, input.UserID, input.UserName, input.CardNumber, input.ExpirationDate, input.SecurityCode, customFields, input.IsFavorite, input.IsArchived)
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
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		profileId := c.GetHeader("X-Profile-Id")

		if profileId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Profile-Id headers are required"})
			return
		}

		var entries []models.VaultEntry
		query := `
			SELECT e.* FROM vault_entries e
			JOIN profiles p ON e.profile_id = p.id
			WHERE e.profile_id = $1 AND p.user_id = $2
			ORDER BY e.name ASC`

		entriesErr := database.Select(&entries, query, profileId, userId)
		if entriesErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve entries"})
			return
		}

		if entries == nil {
			entries = []models.VaultEntry{}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Entries retrieved successfully", "data": entries})
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
		// 1. Extract and validate userId from the JWT (via middleware).
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		// 2. Extract the entryId from the URL path parameter.
		entryId := c.Param("entryId")
		if entryId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entryId path parameter is required"})
			return
		}

		// 3. Extract the profileId from the request header.
		profileId := c.GetHeader("X-Profile-Id")
		if profileId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Profile-Id header is required"})
			return
		}

		// 4. Bind and validate the request body.
		var input VaultEntryInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input: " + err.Error()})
			return
		}

		// 5. Decode the base64-encoded encrypted blob, only if it was provided.
		// Decode the base64-encoded encrypted blob before storing in DB
		encryptedBytes, err := base64.StdEncoding.DecodeString(input.EncryptedBlob)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 encoding for encryptedBlob"})
			return
		}

		// Convert empty string GroupID to nil for the database
		var groupID *uuid.UUID
		if input.GroupID != "" {
			parsedID, err := uuid.Parse(input.GroupID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid GroupID format"})
				return
			}
			groupID = &parsedID
		}

		// Treat an empty/null CustomFields payload as SQL NULL.
		var customFields interface{}
		if len(input.CustomFields) > 0 && string(input.CustomFields) != "null" {
			customFields = input.CustomFields
		}

		// This correctly implements PATCH semantics for every field.
		var updatedEntry models.VaultEntry
		query := `
			UPDATE vault_entries e
			SET
				group_id        = COALESCE($1,  e.group_id),
				type            = COALESCE($2,  e.type::text)::entry_type,
				name            = COALESCE($3,  e.name),
				description     = COALESCE($4,  e.description),
				icon            = COALESCE($5,  e.icon),
				encrypted_blob  = COALESCE($6,  e.encrypted_blob),
				website_url     = COALESCE($7,  e.website_url),
				email           = COALESCE($8,  e.email),
				user_id         = COALESCE($9,  e.user_id),
				user_name       = COALESCE($10, e.user_name),
				card_number     = COALESCE($11, e.card_number),
				expiration_date = COALESCE($12, e.expiration_date),
				security_code   = COALESCE($13, e.security_code),
				custom_fields   = COALESCE($14, e.custom_fields),
				is_favorite     = COALESCE($15, e.is_favorite),
				is_archived     = COALESCE($16, e.is_archived),
				updated_at      = NOW()
			FROM profiles p
			WHERE e.id = $17
			AND e.profile_id = $18
			AND e.profile_id = p.id
			AND p.user_id = $19
			RETURNING e.*`

		dbErr := database.Get(
			&updatedEntry,
			query,
			groupID,
			input.Type,
			input.Name,
			input.Description,
			input.Icon,
			encryptedBytes,
			input.WebsiteURL,
			input.Email,
			input.UserID,
			input.UserName,
			input.CardNumber,
			input.ExpirationDate,
			input.SecurityCode,
			customFields,
			input.IsFavorite,
			input.IsArchived,
			entryId,
			profileId,
			userId,
		)
		if dbErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update entry: " + dbErr.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Entry updated successfully",
			"data":    gin.H{"entry": updatedEntry},
		})
	}
}

// DELETE [/api/v1/entries/:entryId] - removes a vault entry.
func DeleteEntry(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId, err := middleware.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "data": false})
			return
		}

		profileId := c.GetHeader("X-Profile-Id")

		if profileId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "X-Profile-Id headers are required", "data": false})
			return
		}

		entryID := c.Param("entryId")

		query := `
			DELETE FROM vault_entries e
			USING profiles p
			WHERE e.id = $1 AND e.profile_id = p.id AND p.user_id = $2`

		result, err := database.Exec(query, entryID, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete entry", "data": false})
			return
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found or access denied", "data": false})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Entry deleted successfully", "data": true})
	}
}
