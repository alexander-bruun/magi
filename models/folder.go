package models

import "gorm.io/gorm"

type Folder struct {
	gorm.Model
	Name      string `gorm:"not null"`
	LibraryID uint   // Foreign key to Library.ID
}
