package pkg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// deriveKey hashes any-length key string into exactly 32 bytes (AES-256)
// using SHA-256. This means ENCRYPTION_KEY can be any length — short
// passwords, long passphrases, or exactly 32 bytes all work correctly.
func deriveKey(keyStr string) []byte {
	h := sha256.Sum256([]byte(keyStr))
	return h[:]
}

// GenerateAPIKey creates a cryptographically random API key prefixed with "sk-".
// It uses 32 random bytes, hex-encoded, giving a 64-char hex body.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("GenerateAPIKey: rand.Read: %w", err)
	}
	return "sk-" + hex.EncodeToString(b), nil
}

// GenerateRedeemCode creates an 8-byte random code formatted as
// "XXXX-XXXX-XXXX-XXXX" where each X is an uppercase hex digit.
func GenerateRedeemCode() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failures are extremely rare; panic is acceptable here.
		panic(fmt.Sprintf("GenerateRedeemCode: rand.Read: %v", err))
	}
	h := strings.ToUpper(hex.EncodeToString(b)) // 16 uppercase hex chars
	return h[0:4] + "-" + h[4:8] + "-" + h[8:12] + "-" + h[12:16]
}

// HashPassword hashes the given password using bcrypt at cost 12.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("HashPassword: %w", err)
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// keyStr can be any length — it is hashed to 32 bytes via SHA-256.
// The returned value is base64-encoded (nonce prepended to ciphertext).
func Encrypt(plaintext, keyStr string) (string, error) {
	key := deriveKey(keyStr)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Encrypt: NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Encrypt: NewGCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("Encrypt: read nonce: %w", err)
	}

	// Seal appends the ciphertext + tag to nonce.
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt: base64-decodes, splits nonce, then decrypts.
func Decrypt(encoded, keyStr string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("Decrypt: base64 decode: %w", err)
	}

	key := deriveKey(keyStr)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Decrypt: NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Decrypt: NewGCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("Decrypt: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("Decrypt: gcm.Open: %w", err)
	}
	return string(plaintext), nil
}
