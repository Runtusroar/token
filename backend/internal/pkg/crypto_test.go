package pkg

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "sk-") {
		t.Errorf("GenerateAPIKey() = %q, want prefix 'sk-'", key)
	}
	if len(key) <= 30 {
		t.Errorf("GenerateAPIKey() len = %d, want > 30", len(key))
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey() error = %v on iteration %d", err, i)
		}
		if seen[key] {
			t.Fatalf("GenerateAPIKey() produced duplicate key on iteration %d: %q", i, key)
		}
		seen[key] = true
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	password := "s3cr3tP@ssw0rd"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}

	// Correct password must return true.
	if !CheckPassword(password, hash) {
		t.Error("CheckPassword(correct) = false, want true")
	}

	// Wrong password must return false.
	if CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword(wrong) = true, want false")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// 32-byte key for AES-256.
	key := "12345678901234567890123456789012"
	plaintext := "Hello, AI Token Relay!"

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if ciphertext == "" {
		t.Fatal("Encrypt() returned empty ciphertext")
	}
	if ciphertext == plaintext {
		t.Fatal("Encrypt() did not change the text")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestGenerateRedeemCode(t *testing.T) {
	code := GenerateRedeemCode()
	parts := strings.Split(code, "-")
	if len(parts) != 4 {
		t.Fatalf("GenerateRedeemCode() = %q, want 4 parts separated by '-', got %d", code, len(parts))
	}
	for i, p := range parts {
		if len(p) != 4 {
			t.Errorf("GenerateRedeemCode() part[%d] = %q, want 4 chars", i, p)
		}
		// Each character must be an uppercase hex digit.
		for _, ch := range p {
			if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F')) {
				t.Errorf("GenerateRedeemCode() part[%d] = %q contains non-uppercase-hex char %q", i, p, ch)
			}
		}
	}
}
