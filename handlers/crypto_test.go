package handlers

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPepperKey(t *testing.T) {
	rawKey := "my-secret-auth-key-12345"

	// When pepper is empty, returns raw key bytes
	unpeppered := PepperKey(rawKey, "")
	if string(unpeppered) != rawKey {
		t.Fatalf("expected raw key %q, got %q", rawKey, string(unpeppered))
	}

	// When pepper is provided, returns 64-char hex string (SHA256 HMAC)
	pepper := "random-pepper-secret-32-chars-long"
	peppered := PepperKey(rawKey, pepper)
	if len(peppered) != 64 {
		t.Fatalf("expected 64 hex characters for HMAC-SHA256, got length %d: %q", len(peppered), string(peppered))
	}

	// Must be deterministic
	pepperedAgain := PepperKey(rawKey, pepper)
	if string(peppered) != string(pepperedAgain) {
		t.Fatalf("expected deterministic output for identical pepper and key")
	}

	// Different pepper produces different digest
	diffPeppered := PepperKey(rawKey, "different-pepper-value")
	if string(peppered) == string(diffPeppered) {
		t.Fatalf("expected different outputs for different peppers")
	}
}

func TestGeneratePepperedHash_And_Compare(t *testing.T) {
	rawKey := "client-derived-auth-key-sample"
	pepper := "test-pepper-value-123456"

	hash, err := GeneratePepperedHash(rawKey, pepper)
	if err != nil {
		t.Fatalf("unexpected error generating hash: %v", err)
	}

	// 1. Matches with correct pepper and correct key
	matched, needsRehash := ComparePasswordWithPepper(hash, rawKey, pepper)
	if !matched {
		t.Fatalf("expected password to match with correct pepper")
	}
	if needsRehash {
		t.Fatalf("expected needsRehash=false for freshly peppered hash")
	}

	// 2. Fails with wrong key
	matched, _ = ComparePasswordWithPepper(hash, "wrong-key", pepper)
	if matched {
		t.Fatalf("expected failure with wrong key")
	}

	// 3. Fails with wrong pepper
	matched, _ = ComparePasswordWithPepper(hash, rawKey, "wrong-pepper")
	if matched {
		t.Fatalf("expected failure with wrong pepper")
	}
}

func TestComparePasswordWithPepper_LegacyMigration(t *testing.T) {
	rawKey := "legacy-existing-user-auth-key"
	pepper := "new-production-pepper-123"

	// Simulate an existing database record hashed without pepper (legacy bcrypt)
	legacyHashBytes, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate legacy bcrypt hash: %v", err)
	}
	legacyHash := string(legacyHashBytes)

	// Verifying legacy hash when pepper is active:
	// It should match AND signal that rehash is needed!
	matched, needsRehash := ComparePasswordWithPepper(legacyHash, rawKey, pepper)
	if !matched {
		t.Fatalf("expected legacy hash to match candidate key")
	}
	if !needsRehash {
		t.Fatalf("expected needsRehash=true for legacy unpeppered hash")
	}

	// Invalid password on legacy hash must still fail
	matched, _ = ComparePasswordWithPepper(legacyHash, "wrong-password", pepper)
	if matched {
		t.Fatalf("expected failure for wrong password on legacy hash")
	}
}

func TestGetPepper(t *testing.T) {
	os.Unsetenv("PEPPER")
	os.Unsetenv("SECURITY_PEPPER")

	if p := GetPepper(); p != "" {
		t.Fatalf("expected empty pepper when unset, got %q", p)
	}

	os.Setenv("PEPPER", "test-p1")
	if p := GetPepper(); p != "test-p1" {
		t.Fatalf("expected PEPPER env to take precedence, got %q", p)
	}

	os.Unsetenv("PEPPER")
	os.Setenv("SECURITY_PEPPER", "test-p2")
	if p := GetPepper(); p != "test-p2" {
		t.Fatalf("expected SECURITY_PEPPER fallback, got %q", p)
	}
	os.Unsetenv("SECURITY_PEPPER")
}

