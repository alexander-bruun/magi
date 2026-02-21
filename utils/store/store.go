package store

import (
	"io"
	"os"
	"path/filepath"
)

// FileStore provides local file system storage operations.
type FileStore struct {
	basePath string
}

// NewFileStore creates a new local file store rooted at basePath.
func NewFileStore(basePath string) *FileStore {
	return &FileStore{
		basePath: basePath,
	}
}

// Save saves data to the specified path
func (l *FileStore) Save(path string, data []byte) error {
	fullPath := filepath.Join(l.basePath, path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, data, 0644)
}

// SaveReader saves data from a reader to the specified path
func (l *FileStore) SaveReader(path string, reader io.Reader) error {
	fullPath := filepath.Join(l.basePath, path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

// Load loads data from the specified path
func (l *FileStore) Load(path string) ([]byte, error) {
	fullPath := filepath.Join(l.basePath, path)
	return os.ReadFile(fullPath)
}

// LoadReader returns a reader for the specified path
func (l *FileStore) LoadReader(path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.basePath, path)
	return os.Open(fullPath)
}

// Exists checks if a file exists at the specified path
func (l *FileStore) Exists(path string) (bool, error) {
	fullPath := filepath.Join(l.basePath, path)
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Delete deletes a file at the specified path
func (l *FileStore) Delete(path string) error {
	fullPath := filepath.Join(l.basePath, path)
	return os.Remove(fullPath)
}

// CreateDir creates a directory at the specified path
func (l *FileStore) CreateDir(path string) error {
	fullPath := filepath.Join(l.basePath, path)
	return os.MkdirAll(fullPath, 0755)
}

// List lists files in the specified directory
func (l *FileStore) List(path string) ([]string, error) {
	fullPath := filepath.Join(l.basePath, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}
