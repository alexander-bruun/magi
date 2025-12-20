package handlers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopyFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := ioutil.TempDir("", "copyfile_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := "Hello, World!"
	err = ioutil.WriteFile(srcPath, []byte(srcContent), 0644)
	assert.NoError(t, err)

	// Set source file permissions
	err = os.Chmod(srcPath, 0755)
	assert.NoError(t, err)

	// Copy file
	dstPath := filepath.Join(tempDir, "destination.txt")
	err = copyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify content
	dstContent, err := ioutil.ReadFile(dstPath)
	assert.NoError(t, err)
	assert.Equal(t, srcContent, string(dstContent))

	// Verify permissions
	srcInfo, err := os.Stat(srcPath)
	assert.NoError(t, err)
	dstInfo, err := os.Stat(dstPath)
	assert.NoError(t, err)
	assert.Equal(t, srcInfo.Mode(), dstInfo.Mode())
}

func TestCopyFileNonExistentSource(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "copyfile_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	srcPath := filepath.Join(tempDir, "nonexistent.txt")
	dstPath := filepath.Join(tempDir, "destination.txt")

	err = copyFile(srcPath, dstPath)
	assert.Error(t, err)
}

func TestCopyFileDestinationExists(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "copyfile_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := "Source content"
	err = ioutil.WriteFile(srcPath, []byte(srcContent), 0644)
	assert.NoError(t, err)

	// Create destination file
	dstPath := filepath.Join(tempDir, "destination.txt")
	dstContent := "Destination content"
	err = ioutil.WriteFile(dstPath, []byte(dstContent), 0644)
	assert.NoError(t, err)

	// Copy file (should overwrite)
	err = copyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify content was overwritten
	newDstContent, err := ioutil.ReadFile(dstPath)
	assert.NoError(t, err)
	assert.Equal(t, srcContent, string(newDstContent))
}