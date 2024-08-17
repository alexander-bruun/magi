package models

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.etcd.io/bbolt"
)

type JWTKey struct {
	Key string `json:"key"`
}

// GenerateRandomKey creates a new random key of the specified length
func GenerateRandomKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// StoreKey saves the JWT key to the database
func StoreKey(key string) error {
	return updateBucket("jwt", "jwt_key", JWTKey{Key: key})
}

// GetKey retrieves the JWT key from the database
func GetKey() (string, error) {
	var jwtKey JWTKey
	err := getFromBucket("jwt", "jwt_key", &jwtKey)
	if err != nil {
		return "", err
	}
	return jwtKey.Key, nil
}

// CreateAccessToken generates a new access token with a 15-minute expiry
func CreateAccessToken(userName string) (string, error) {
	return createToken(userName, nil, 15*time.Minute)
}

// CreateRefreshToken generates a new refresh token with a 7-day expiry
func CreateRefreshToken(userName string, version int) (string, error) {
	claims := jwt.MapClaims{
		"user_name": userName,
		"version":   version,
	}
	return createToken(userName, claims, 7*24*time.Hour)
}

// ValidateToken validates a token and returns its claims
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	secret, err := GetKey()
	if err != nil {
		return nil, errors.New("failed to get secret")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return handleTokenValidationError(err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("token invalid")
}

// RefreshAccessToken generates a new access token from a valid refresh token
func RefreshAccessToken(refreshToken string) (string, string, error) {
	claims, err := ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	userName, version := claims["user_name"].(string), int(claims["version"].(float64))
	user, err := FindUserByUsername(userName)
	if err != nil || user.RefreshTokenVersion != version {
		return "", "", errors.New("invalid refresh token version")
	}

	newAccessToken, err := CreateAccessToken(userName)
	if err != nil {
		return "", "", err
	}
	return newAccessToken, userName, nil
}

// GenerateNewRefreshToken creates a new refresh token and updates the user's version
func GenerateNewRefreshToken(userName string) (string, error) {
	user, err := FindUserByUsername(userName)
	if err != nil {
		return "", errors.New("user not found")
	}

	if err := IncrementRefreshTokenVersion(user.Username); err != nil {
		return "", errors.New("failed to increment refresh token version")
	}

	return CreateRefreshToken(userName, user.RefreshTokenVersion)
}

// createToken generates a JWT token with specified claims and expiry duration
func createToken(userName string, additionalClaims jwt.MapClaims, expiry time.Duration) (string, error) {
	secret, err := GetKey()
	if err != nil {
		return "", errors.New("failed to get secret")
	}

	claims := jwt.MapClaims{
		"user_name": userName,
		"exp":       time.Now().Add(expiry).Unix(),
	}

	// Add additional claims if provided
	for k, v := range additionalClaims {
		claims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// handleTokenValidationError interprets JWT validation errors
func handleTokenValidationError(err error) (jwt.MapClaims, error) {
	if ve, ok := err.(*jwt.ValidationError); ok {
		if ve.Errors&jwt.ValidationErrorExpired != 0 {
			return nil, errors.New("token expired")
		}
		return nil, errors.New("token invalid")
	}
	return nil, err
}

// Helper functions for bucket operations

func updateBucket(bucket, key string, data interface{}) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), encoded)
	})
}

func getFromBucket(bucket, key string, data interface{}) error {
	return db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(key))
		if v == nil {
			return errors.New("key not found")
		}
		return json.Unmarshal(v, data)
	})
}
