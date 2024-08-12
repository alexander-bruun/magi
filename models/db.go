package models

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alexander-bruun/magi/utils"
	"go.etcd.io/bbolt"
)

var db *bbolt.DB

// Initialize connects to the BoltDB database and creates necessary buckets
func Initialize(cacheDirectory string) error {
	start := time.Now()
	defer utils.LogDuration("Initialize", start)

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
	start := time.Now()
	defer utils.LogDuration("Close", start)

	if db != nil {
		return db.Close()
	}
	return nil
}

// Helper functions for CRUD operations

func create(bucket string, slug string, data interface{}) error {
	start := time.Now()
	defer utils.LogDuration("create", start, bucket, slug)

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
	start := time.Now()
	defer utils.LogDuration("get", start, bucket, slug)

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
	start := time.Now()
	defer utils.LogDuration("update", start, bucket, slug)

	return create(bucket, slug, data)
}

func delete(bucket string, slug string) error {
	start := time.Now()
	defer utils.LogDuration("delete", start, bucket, slug)

	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.Delete([]byte(slug))
	})
}

func deleteKeysWithPattern(bucket string, pattern string) error {
	start := time.Now()
	defer utils.LogDuration("deleteKeysWithPattern", start, bucket, pattern)

	// Convert wildcard pattern to regular expression
	regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`) + "$"
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return fmt.Errorf("compile regex: %s", err)
	}

	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}

		// Iterate over all keys in the bucket
		return b.ForEach(func(k, v []byte) error {
			if re.Match(k) {
				// Delete key if it matches the pattern
				return b.Delete(k)
			}
			return nil
		})
	})
}

func getAll(bucket string, dataList *[]([]byte)) error {
	start := time.Now()
	defer utils.LogDuration("getAll", start, bucket)

	// Clear the existing slice
	*dataList = (*dataList)[:0]

	return db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.ForEach(func(k, v []byte) error {
			*dataList = append(*dataList, v)
			return nil
		})
	})
}

func exists(bucket string, key string) (bool, error) {
	start := time.Now()
	defer utils.LogDuration("exists", start, bucket, key)

	var exists bool
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}
		v := b.Get([]byte(key))
		if v != nil {
			exists = true
		}
		return nil
	})
	return exists, err
}

// getAllKeys retrieves all keys in the specified bucket.
func getAllKeys(bucket string) ([]string, error) {
	start := time.Now()
	defer utils.LogDuration("getAllKeys", start, bucket)

	var keys []string

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}

		return b.ForEach(func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return keys, nil
}
