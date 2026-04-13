package pkg

import (
	"testing"
	"time"
)

func TestJWT_SignAndVerify(t *testing.T) {
	secret := "super-secret-key"
	claims := &JWTClaims{
		UserID: 42,
		Email:  "user@example.com",
		Role:   "admin",
	}

	token, err := SignJWT(claims, secret, 15*time.Minute)
	if err != nil {
		t.Fatalf("SignJWT() error = %v", err)
	}
	if token == "" {
		t.Fatal("SignJWT() returned empty token")
	}

	parsed, err := VerifyJWT(token, secret)
	if err != nil {
		t.Fatalf("VerifyJWT() error = %v", err)
	}
	if parsed.UserID != claims.UserID {
		t.Errorf("UserID = %d, want %d", parsed.UserID, claims.UserID)
	}
	if parsed.Email != claims.Email {
		t.Errorf("Email = %q, want %q", parsed.Email, claims.Email)
	}
	if parsed.Role != claims.Role {
		t.Errorf("Role = %q, want %q", parsed.Role, claims.Role)
	}
}

func TestJWT_Expired(t *testing.T) {
	secret := "super-secret-key"
	claims := &JWTClaims{
		UserID: 1,
		Email:  "exp@example.com",
		Role:   "user",
	}

	// Negative TTL produces a token that is already expired.
	token, err := SignJWT(claims, secret, -1*time.Second)
	if err != nil {
		t.Fatalf("SignJWT() error = %v", err)
	}

	_, err = VerifyJWT(token, secret)
	if err == nil {
		t.Fatal("VerifyJWT() expected error for expired token, got nil")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	claims := &JWTClaims{
		UserID: 7,
		Email:  "wrong@example.com",
		Role:   "user",
	}

	token, err := SignJWT(claims, "secret-one", 15*time.Minute)
	if err != nil {
		t.Fatalf("SignJWT() error = %v", err)
	}

	_, err = VerifyJWT(token, "secret-two")
	if err == nil {
		t.Fatal("VerifyJWT() expected error for wrong secret, got nil")
	}
}
