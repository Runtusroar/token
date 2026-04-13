package pkg

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims holds the application-specific claims embedded in a JWT.
type JWTClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// SignJWT creates and signs a JWT with the given claims using HS256.
// The token expires after ttl from now.
func SignJWT(claims *JWTClaims, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("SignJWT: %w", err)
	}
	return signed, nil
}

// VerifyJWT parses and validates a JWT string, returning the embedded claims.
// It returns an error if the token is invalid, expired, or signed with a
// different secret.
func VerifyJWT(tokenStr, secret string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("VerifyJWT: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("VerifyJWT: token is not valid")
	}
	return claims, nil
}
