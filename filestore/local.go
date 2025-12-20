package filestore

import (
	"io"
	"os"
	"path/filepath"
)

// LocalFileSystemAdapter implements CacheBackend for local file system storage
type LocalFileSystemAdapter struct {
	basePath string
}

// NewLocalFileSystemAdapter creates a new local file system cache adapter
func NewLocalFileSystemAdapter(basePath string) *LocalFileSystemAdapter {
	return &LocalFileSystemAdapter{
		basePath: basePath,
	}
}

// Save saves data to the specified path
func (l *LocalFileSystemAdapter) Save(path string, data []byte) error {
	fullPath := filepath.Join(l.basePath, path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, data, 0644)
}

// SaveReader saves data from a reader to the specified path
func (l *LocalFileSystemAdapter) SaveReader(path string, reader io.Reader) error {
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
func (l *LocalFileSystemAdapter) Load(path string) ([]byte, error) {
	fullPath := filepath.Join(l.basePath, path)
	return os.ReadFile(fullPath)
}

// LoadReader returns a reader for the specified path
func (l *LocalFileSystemAdapter) LoadReader(path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.basePath, path)
	return os.Open(fullPath)
}

// Exists checks if a file exists at the specified path
func (l *LocalFileSystemAdapter) Exists(path string) (bool, error) {
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
func (l *LocalFileSystemAdapter) Delete(path string) error {
	fullPath := filepath.Join(l.basePath, path)
	return os.Remove(fullPath)
}

// CreateDir creates a directory at the specified path
func (l *LocalFileSystemAdapter) CreateDir(path string) error {
	fullPath := filepath.Join(l.basePath, path)
	return os.MkdirAll(fullPath, 0755)
}

// List lists files in the specified directory
func (l *LocalFileSystemAdapter) List(path string) ([]string, error) {
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