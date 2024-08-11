package models

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

var db *bbolt.DB

// Initialize connects to the BoltDB database and creates necessary buckets
func Initialize(cacheDirectory string) error {
	databasePath := filepath.Join(cacheDirectory, "magi.db")

	var err error
	db, err = bbolt.Open(databasePath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}

	// Create buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{"libraries", "mangas", "chapters", "users", "jwt"}
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
		}
		return nil
	})

	return err
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// Helper functions for CRUD operations

func create(bucket string, slug string, data interface{}) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		encoded, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return b.Put([]byte(slug), encoded)
	})
}

func get(bucket string, slug string, data interface{}) error {
	return db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(slug))
		if v == nil {
			return bbolt.ErrBucketNotFound
		}
		return json.Unmarshal(v, data)
	})
}

func update(bucket string, slug string, data interface{}) error {
	return create(bucket, slug, data)
}

func delete(bucket string, slug string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.Delete([]byte(slug))
	})
}

func getAll(bucket string) ([][]byte, error) {
	var dataList [][]byte
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.ForEach(func(k, v []byte) error {
			dataList = append(dataList, v)
			return nil
		})
	})
	return dataList, err
}
