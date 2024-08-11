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
	Key string
}

func GenerateRandomKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func StoreKey(key string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("jwt"))
		jwtKey := JWTKey{Key: key}
		encoded, err := json.Marshal(jwtKey)
		if err != nil {
			return err
		}
		return b.Put([]byte("jwt_key"), encoded)
	})
}

func GetKey() (string, error) {
	var key JWTKey
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("jwt"))
		v := b.Get([]byte("jwt_key"))
		if v == nil {
			return errors.New("JWT key not found")
		}
		return json.Unmarshal(v, &key)
	})
	if err != nil {
		return "", err
	}
	return key.Key, nil
}

// CreateAccessToken generates a new access token
func CreateAccessToken(userName string) (string, error) {
	secret, err := GetKey()
	if err != nil {
		return "", errors.New("failed to get secret")
	}
	claims := jwt.MapClaims{
		"user_name": userName,
		"exp":       time.Now().Add(time.Minute * 15).Unix(), // Access token expires in 15 minutes
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// CreateRefreshToken generates a new refresh token including the version
func CreateRefreshToken(userName string, version int) (string, error) {
	secret, err := GetKey()
	if err != nil {
		return "", errors.New("failed to get secret")
	}
	claims := jwt.MapClaims{
		"user_name": userName,
		"version":   version,
		"exp":       time.Now().Add(time.Hour * 24 * 7).Unix(), // Refresh token expires in 7 days
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken validates a token and returns the claims
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	secret, err := GetKey()
	if err != nil {
		return nil, errors.New("failed to get secret")
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, errors.New("token expired")
			}
			return nil, errors.New("token invalid")
		}
		return nil, err
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

	userName := claims["user_name"].(string)
	version := int(claims["version"].(float64))

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

// GenerateNewRefreshToken generates a new refresh token and updates the user's version
func GenerateNewRefreshToken(userName string) (string, error) {
	user, err := FindUserByUsername(userName)
	if err != nil {
		return "", errors.New("user not found")
	}

	err = IncrementRefreshTokenVersion(user)
	if err != nil {
		return "", errors.New("failed to increment refresh token version")
	}

	return CreateRefreshToken(userName, user.RefreshTokenVersion)
}
