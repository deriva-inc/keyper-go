package models

import (
	"time"

	"github.com/google/uuid"
)

// User corresponds to the 'users' table
type User struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	AuthHash     string    `db:"auth_hash" json:"authHash"`
	Salt         string    `db:"salt" json:"salt"`
	RecoveryHash *string   `db:"recovery_hash" json:"recoveryHash,omitempty"`
	DisplayName  *string   `db:"display_name" json:"displayName,omitempty"`
	AvatarURL    *string   `db:"avatar_url" json:"avatarUrl,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

// Profile corresponds to the 'profiles' table
type Profile struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"userId"`
	Name      string    `db:"name" json:"name"`
	Icon      *string   `db:"icon" json:"icon,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// Group corresponds to the 'groups' table
type Group struct {
	ID        uuid.UUID `db:"id" json:"id"`
	ProfileID uuid.UUID `db:"profile_id" json:"profileId"`
	Name      string    `db:"name" json:"name"`
	Icon      *string   `db:"icon" json:"icon,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// VaultEntry corresponds to the 'vault_entries' table
type VaultEntry struct {
	ID            uuid.UUID  `db:"id" json:"id"`
	ProfileID     uuid.UUID  `db:"profile_id" json:"profileId"`
	GroupID       *uuid.UUID `db:"group_id" json:"groupId"`
	Type          string     `db:"type" json:"type"`
	Name          string     `db:"name" json:"name"`
	EncryptedBlob []byte     `db:"encrypted_blob" json:"encryptedBlob"`
	CustomFields  *string    `db:"custom_fields" json:"customFields,omitempty"`
	IsFavorite    bool       `db:"is_favorite" json:"isFavorite"`
	CreatedAt     time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updatedAt"`
}
