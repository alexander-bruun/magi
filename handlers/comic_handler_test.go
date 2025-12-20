package handlers

import (
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/alexander-bruun/magi/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetImagesFromDirectory(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "images_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	files := []string{
		"03.jpg",
		"01.png",
		"02.jpeg",
		"text.txt", // should be ignored
		"04.gif",
	}

	for _, file := range files {
		path := filepath.Join(tempDir, file)
		err := os.WriteFile(path, []byte("fake image"), 0644)
		assert.NoError(t, err)
	}

	tests := []struct {
		page     int
		expected string
		hasError bool
	}{
		{1, filepath.Join(tempDir, "01.png"), false},
		{2, filepath.Join(tempDir, "02.jpeg"), false},
		{3, filepath.Join(tempDir, "03.jpg"), false},
		{4, filepath.Join(tempDir, "04.gif"), false},
		{0, "", true}, // page out of range
		{5, "", true}, // page out of range
	}

	for _, tt := range tests {
		result, err := GetImagesFromDirectory(tempDir, tt.page)
		if tt.hasError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		}
	}
}

func TestGetImagesFromDirectoryEmpty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "images_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// No image files
	_, err = GetImagesFromDirectory(tempDir, 1)
	assert.Error(t, err)
}

func TestGetImagesFromDirectoryNonExistent(t *testing.T) {
	_, err := GetImagesFromDirectory("/non/existent/path", 1)
	assert.Error(t, err)
}

func TestGetCompressionQualityForUser(t *testing.T) {
	// Test with empty username (anonymous)
	quality := GetCompressionQualityForUser("")
	assert.Equal(t, 70, quality) // anonymous quality

	// Test with non-existent user (should fallback to reader)
	quality = GetCompressionQualityForUser("nonexistent")
	assert.Equal(t, 70, quality) // reader quality as fallback
}

func TestProcessImageForServing(t *testing.T) {
	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	// Save test image
	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "test.jpg")
	file, err := os.Create(imagePath)
	assert.NoError(t, err)
	defer file.Close()

	err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	assert.NoError(t, err)

	// Test processing with quality 80
	result, err := ProcessImageForServing(imagePath, 80)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, len(result) > 0)

	// Test with invalid file
	_, err = ProcessImageForServing("/non/existent/file.jpg", 80)
	assert.Error(t, err)

	// Test with invalid image file (create a text file)
	textPath := filepath.Join(tempDir, "text.txt")
	err = os.WriteFile(textPath, []byte("not an image"), 0644)
	assert.NoError(t, err)

	_, err = ProcessImageForServing(textPath, 80)
	assert.Error(t, err)
}

func TestServeComicArchiveFromZIP(t *testing.T) {
	// Test with non-existent file
	_, err := ServeComicArchiveFromZIP("/non/existent.zip", 1, 80)
	assert.Error(t, err)

	// Test with invalid page
	_, err = ServeComicArchiveFromZIP("/non/existent.zip", 0, 80)
	assert.Error(t, err)
}

func TestServeComicArchiveFromRAR(t *testing.T) {
	// Test with non-existent file
	_, err := ServeComicArchiveFromRAR("/non/existent.rar", 1, 80)
	assert.Error(t, err)

	// Test with invalid page
	_, err = ServeComicArchiveFromRAR("/non/existent.rar", 0, 80)
	assert.Error(t, err)
}

func TestComicHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/comic", ComicHandler)

	// Test missing token
	req := httptest.NewRequest("GET", "/comic", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	// Test invalid token
	req = httptest.NewRequest("GET", "/comic?token=invalid", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)

	// Test valid token but media/chapter not found
	token := utils.GenerateImageAccessToken("nonexistent", "chapter1", 1)
	req = httptest.NewRequest("GET", "/comic?token="+token, nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode) // Media not found
}

func TestServeImageFromDirectory(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return serveImageFromDirectory(c, "/non/existent", 1)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	// Should return error view, which is 200 with error content
	assert.Equal(t, 200, resp.StatusCode)
}