package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User corresponds to the 'users' table
type User struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Email          string    `db:"email" json:"email"`
	AuthKey        string    `db:"auth_key" json:"authKey"`
	AuthSalt       string    `db:"auth_salt" json:"authSalt"`
	EncryptionKey  string    `db:"encryption_key" json:"encryptionKey"`
	EncryptionSalt string    `db:"encryption_salt" json:"encryptionSalt"`
	RecoveryKey    *string   `db:"recovery_hash" json:"recoveryKey,omitempty"`
	Name           *string   `db:"name" json:"name,omitempty"`
	AvatarURL      *string   `db:"avatar_url" json:"avatarUrl,omitempty"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `db:"updated_at" json:"updatedAt"`
}

// Profile corresponds to the 'profiles' table
type Profile struct {
	ID          uuid.UUID `db:"id" json:"id"`
	UserID      uuid.UUID `db:"user_id" json:"userId"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	Icon        *string   `db:"icon" json:"icon,omitempty"`
	IsArchived  bool      `db:"is_archived" json:"isArchived"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

// Group corresponds to the 'groups' table
type Group struct {
	ID          uuid.UUID `db:"id" json:"id"`
	ProfileID   uuid.UUID `db:"profile_id" json:"profileId"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	Type        string    `db:"type" json:"type"`
	Icon        *string   `db:"icon" json:"icon,omitempty"`
	IsArchived  bool      `db:"is_archived" json:"isArchived"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

// VaultEntry corresponds to the 'vault_entries' table
type VaultEntry struct {
	ID             uuid.UUID       `db:"id" json:"id"`
	ProfileID      uuid.UUID       `db:"profile_id" json:"profileId"`
	GroupID        *uuid.UUID      `db:"group_id" json:"groupId"`
	Type           string          `db:"type" json:"type"`
	Name           string          `db:"name" json:"name"`
	Description    *string         `db:"description" json:"description,omitempty"`
	Icon           *string         `db:"icon" json:"icon,omitempty"`
	EncryptedBlob  []byte          `db:"encrypted_blob" json:"encryptedBlob"`
	CustomFields   json.RawMessage `db:"custom_fields" json:"customFields,omitempty"`
	WebsiteURL     *string         `db:"website_url" json:"websiteUrl,omitempty"`
	Email          *string         `db:"email" json:"email,omitempty" binding:"required"`
	UserID         *string         `db:"user_id" json:"userId,omitempty"`
	UserName       *string         `db:"user_name" json:"userName,omitempty"`
	CardNumber     *string         `db:"card_number" json:"cardNumber,omitempty"`
	ExpirationDate *string         `db:"expiration_date" json:"expirationDate,omitempty"`
	SecurityCode   *string         `db:"security_code" json:"securityCode,omitempty"`
	IsFavorite     bool            `db:"is_favorite" json:"isFavorite"`
	IsArchived     bool            `db:"is_archived" json:"isArchived"`
	CreatedAt      time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time       `db:"updated_at" json:"updatedAt"`
}
