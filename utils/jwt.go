package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Claims defines the JWT claims
type Claims struct {
	Username            string `json:"username"`
	TokenType           string `json:"token_type"`
	RefreshTokenVersion uint   `json:"refresh_token_version"`
	jwt.RegisteredClaims
}

var jwtKey []byte // This will be set from outside the package

// SetJWTKey sets the JWT key to be used for signing and validating tokens
func SetJWTKey(key string) {
	jwtKey = []byte(key)
}

// GenerateToken creates a new JWT token with the specified claims
func GenerateToken(username string, tokenType string, refreshTokenVersion uint, expirationTime time.Time) (string, error) {
	claims := &Claims{
		Username:            username,
		TokenType:           tokenType,
		RefreshTokenVersion: refreshTokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ValidateToken parses and validates a JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

// RefreshToken generates new access and refresh tokens
func RefreshToken(refreshToken string, currentRefreshTokenVersion uint) (string, string, error) {
	claims, err := ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	if claims.TokenType != "refresh" {
		return "", "", errors.New("invalid token type")
	}

	if claims.RefreshTokenVersion != currentRefreshTokenVersion {
		return "", "", errors.New("refresh token version mismatch")
	}

	accessExpirationTime := time.Now().Add(15 * time.Minute)
	refreshExpirationTime := time.Now().Add(30 * 24 * time.Hour)

	newAccessToken, err := GenerateToken(claims.Username, "access", currentRefreshTokenVersion, accessExpirationTime)
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := GenerateToken(claims.Username, "refresh", currentRefreshTokenVersion, refreshExpirationTime)
	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}
