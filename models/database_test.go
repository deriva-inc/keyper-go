package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUserModel_JSONSerialization(t *testing.T) {
	id := uuid.New()
	name := "Alice"
	now := time.Now().Truncate(time.Second)

	user := User{
		ID:             id,
		Email:          "alice@example.com",
		AuthKey:        "hashed-auth-key",
		AuthSalt:       "auth-salt",
		EncryptionSalt: "encryption-salt",
		Name:           &name,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal user: %v", err)
	}

	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal user: %v", err)
	}

	if decoded.Email != user.Email || decoded.ID != user.ID {
		t.Errorf("decoded user mismatch: expected %v, got %v", user, decoded)
	}
}

func TestProfileModel_JSONSerialization(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	desc := "Work Profile"

	profile := Profile{
		ID:          id,
		UserID:      userID,
		Name:        "Work",
		Description: &desc,
		IsArchived:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("failed to marshal profile: %v", err)
	}

	var decoded Profile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal profile: %v", err)
	}

	if decoded.Name != "Work" || decoded.UserID != userID {
		t.Errorf("decoded profile mismatch")
	}
}

func TestVaultEntryModel_JSONSerialization(t *testing.T) {
	id := uuid.New()
	profileID := uuid.New()
	email := "test@vault.com"

	entry := VaultEntry{
		ID:            id,
		ProfileID:     profileID,
		Type:          "login",
		Name:          "GitHub",
		Email:         &email,
		EncryptedBlob: []byte("encrypted-secret-payload"),
		IsFavorite:    true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal entry: %v", err)
	}

	var decoded VaultEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	if decoded.Name != "GitHub" || *decoded.Email != email {
		t.Errorf("decoded entry mismatch")
	}
}

