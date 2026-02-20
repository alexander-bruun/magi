package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// GetImagesFromDirectory gets image files from a directory
func GetImagesFromDirectory(dirPath string, page int) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	var imageFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if isImageFile(name) {
				imageFiles = append(imageFiles, name)
			}
		}
	}

	sort.Strings(imageFiles)

	if page < 1 || page > len(imageFiles) {
		return "", fmt.Errorf("page %d out of range", page)
	}

	return filepath.Join(dirPath, imageFiles[page-1]), nil
}
