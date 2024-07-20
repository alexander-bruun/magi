package models

import (
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

// Initialize connects to the SQLite database and performs auto migrations
func Initialize(cacheDirectory string) error {
	gormConfig := &gorm.Config{
		CreateBatchSize:        1500,
		SkipDefaultTransaction: true,
	}

	databasePath := filepath.Join(cacheDirectory, "magi.db")

	var err error
	db, err = gorm.Open(
		sqlite.Open(databasePath),
		gormConfig,
	)
	if err != nil {
		return err
	}

	// Auto migrations for models
	err = db.AutoMigrate(&Library{},
		&Manga{},
		&Chapter{},
		&Folder{})
	if err != nil {
		return err
	}

	// Return nil on successful initialization
	return nil
}

// Close closes the database connection
func Close() error {
	if db == nil {
		return nil // If db is nil, return early
	}

	db, err := db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
