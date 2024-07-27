package models

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"gorm.io/gorm"
)

type JWTKey struct {
	ID  uint `gorm:"primaryKey"`
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
	return db.Transaction(func(tx *gorm.DB) error {
		var existingKey JWTKey
		if err := tx.First(&existingKey).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if existingKey.ID > 0 {
			existingKey.Key = key
			return tx.Save(&existingKey).Error
		}
		newKey := JWTKey{Key: key}
		return tx.Create(&newKey).Error
	})
}

func GetKey() (string, error) {
	var key JWTKey
	if err := db.First(&key).Error; err != nil {
		return "", err
	}
	return key.Key, nil
}
