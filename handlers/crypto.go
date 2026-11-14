package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// GetPepper returns the pepper secret from the environment.
func GetPepper() string {
	if p := os.Getenv("PEPPER"); p != "" {
		return p
	}
	if p := os.Getenv("SECURITY_PEPPER"); p != "" {
		return p
	}
	return ""
}

// PepperKey hashes rawKey using HMAC-SHA256 with the provided pepper.
// If pepper is empty, rawKey is returned as a byte slice.
func PepperKey(rawKey string, pepper string) []byte {
	if pepper == "" {
		return []byte(rawKey)
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(rawKey))
	return []byte(hex.EncodeToString(mac.Sum(nil)))
}

// GeneratePepperedHash creates a bcrypt hash of rawKey using the pepper.
func GeneratePepperedHash(rawKey string, pepper string) (string, error) {
	pepperedBytes := PepperKey(rawKey, pepper)
	hash, err := bcrypt.GenerateFromPassword(pepperedBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePasswordWithPepper verifies a candidate rawKey against a bcrypt hash.
// It supports seamless backward-compatibility:
// 1. First tests against the peppered key.
// 2. If mismatched and pepper is configured, tests against the legacy unpeppered key.
// Returns (matched, needsRehash).
func ComparePasswordWithPepper(hashedPassword string, rawKey string, pepper string) (matched bool, needsRehash bool) {
	// 1. Try peppered comparison
	pepperedBytes := PepperKey(rawKey, pepper)
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), pepperedBytes); err == nil {
		return true, false
	}

	// 2. If pepper is active, fallback to checking legacy unpeppered key
	if pepper != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(rawKey)); err == nil {
			return true, true // Matched legacy unpeppered hash! Rehash recommended.
		}
	}

	return false, false
}

