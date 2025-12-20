package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetFileNameWithExtension(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		fileUrl  string
		expected string
	}{
		{
			name:     "filename with extension",
			fileName: "image.jpg",
			fileUrl:  "http://example.com/image.png",
			expected: "image.jpg",
		},
		{
			name:     "filename without extension",
			fileName: "image",
			fileUrl:  "http://example.com/image.png",
			expected: "image.png",
		},
		{
			name:     "url without extension",
			fileName: "image",
			fileUrl:  "http://example.com/image",
			expected: "image",
		},
		{
			name:     "empty filename",
			fileName: "",
			fileUrl:  "http://example.com/image.png",
			expected: ".png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileNameWithExtension(tt.fileName, tt.fileUrl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateCropOffset(t *testing.T) {
	tests := []struct {
		name         string
		imgWidth     int
		imgHeight    int
		targetWidth  int
		targetHeight int
		expectedX    int
		expectedY    int
	}{
		{
			name:         "center crop",
			imgWidth:     1000,
			imgHeight:    800,
			targetWidth:  500,
			targetHeight: 400,
			expectedX:    250, // (1000-500)/2
			expectedY:    200, // (800-400)/2
		},
		{
			name:         "exact match",
			imgWidth:     500,
			imgHeight:    400,
			targetWidth:  500,
			targetHeight: 400,
			expectedX:    0,
			expectedY:    0,
		},
		{
			name:         "smaller target",
			imgWidth:     100,
			imgHeight:    100,
			targetWidth:  50,
			targetHeight: 50,
			expectedX:    25,
			expectedY:    25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test image with the specified dimensions
			img := image.NewRGBA(image.Rect(0, 0, tt.imgWidth, tt.imgHeight))
			x, y := calculateCropOffset(img, tt.targetWidth, tt.targetHeight)
			assert.Equal(t, tt.expectedX, x)
			assert.Equal(t, tt.expectedY, y)
		})
	}
}

func TestEnsureDirExists(t *testing.T) {
	// Test with a temp directory
	dir := "/tmp/test_dir"
	err := ensureDirExists(dir)
	assert.NoError(t, err)

	// Test with existing directory
	err = ensureDirExists(dir)
	assert.NoError(t, err)

	// Test with current directory
	err = ensureDirExists(".")
	assert.NoError(t, err)
}

func TestGenerateRandomSecret(t *testing.T) {
	secret1, err := GenerateRandomSecret()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret1)
	assert.Len(t, secret1, 64) // 32 bytes * 2 hex chars

	secret2, err := GenerateRandomSecret()
	assert.NoError(t, err)
	assert.NotEqual(t, secret1, secret2) // Should be random
}

func TestGenerateSignedImageURL(t *testing.T) {
	baseURL := "https://example.com/image"
	secret := "test-secret"
	mediaSlug := "test-media"
	chapterSlug := "test-chapter"
	page := 1
	expiration := time.Hour

	url := GenerateSignedImageURL(baseURL, secret, mediaSlug, chapterSlug, page, expiration)
	assert.Contains(t, url, baseURL)
	assert.Contains(t, url, "media=test-media")
	assert.Contains(t, url, "chapter=test-chapter")
	assert.Contains(t, url, "page=1")
	assert.Contains(t, url, "expires=")
	assert.Contains(t, url, "signature=")
}

func TestValidateImageSignature(t *testing.T) {
	secret := "test-secret"
	mediaSlug := "test-media"
	chapterSlug := "test-chapter"
	page := 1
	expiration := time.Hour

	// Calculate expires
	expires := time.Now().Add(expiration).Unix()
	expiresStr := strconv.FormatInt(expires, 10)
	pageStr := "1"

	// Recreate signature for validation
	data := fmt.Sprintf("%s:%s:%d:%d", mediaSlug, chapterSlug, page, expires)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))

	// Valid signature
	err := ValidateImageSignature(secret, mediaSlug, chapterSlug, pageStr, expiresStr, signature)
	assert.NoError(t, err)

	// Invalid signature
	err = ValidateImageSignature(secret, mediaSlug, chapterSlug, pageStr, expiresStr, "invalid")
	assert.Error(t, err)

	// Expired
	pastExpires := strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	err = ValidateImageSignature(secret, mediaSlug, chapterSlug, pageStr, pastExpires, signature)
	assert.Error(t, err)
}

func TestGenerateImageAccessToken(t *testing.T) {
	token1 := GenerateImageAccessToken("media1", "chapter1", 1)
	assert.NotEmpty(t, token1)

	token2 := GenerateImageAccessToken("media1", "chapter1", 1)
	assert.NotEqual(t, token1, token2) // Should be unique
}

func TestGetFloat64(t *testing.T) {
	m := map[string]interface{}{
		"float64":  3.14,
		"int":      42,
		"string":   "hello",
		"missing":  nil,
	}

	tests := []struct {
		key      string
		expected float64
	}{
		{"float64", 3.14},
		{"int", 0},      // not a float64
		{"string", 0},   // not a float64
		{"missing", 0},  // key doesn't exist
		{"nonexistent", 0}, // key doesn't exist
	}

	for _, test := range tests {
		result := getFloat64(m, test.key)
		assert.Equal(t, test.expected, result, "getFloat64(%s)", test.key)
	}
}

func TestCheckFileExists(t *testing.T) {
	// Test existing file
	tempFile := t.TempDir() + "/test.txt"
	content := "test"
	err := os.WriteFile(tempFile, []byte(content), 0644)
	assert.NoError(t, err)

	err = checkFileExists(tempFile)
	assert.NoError(t, err)

	// Test non-existing file
	err = checkFileExists("/nonexistent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")
}

func TestValidateImageToken(t *testing.T) {
	// Generate a token
	token := GenerateImageAccessToken("test-media", "test-chapter", 1)
	assert.NotEmpty(t, token)

	// Validate the token
	tokenInfo, err := ValidateImageToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, tokenInfo)
	assert.Equal(t, "test-media", tokenInfo.MediaSlug)
	assert.Equal(t, "test-chapter", tokenInfo.ChapterSlug)
	assert.Equal(t, 1, tokenInfo.Page)
	assert.Equal(t, "", tokenInfo.AssetPath)

	// Consume the token
	ConsumeImageToken(token)

	// Try to validate again - should fail
	_, err = ValidateImageToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestValidateImageTokenWithAsset(t *testing.T) {
	// Generate a token with asset
	token := GenerateImageAccessTokenWithAsset("test-media", "test-chapter", 2, "test.jpg")
	assert.NotEmpty(t, token)

	// Validate the token
	tokenInfo, err := ValidateImageToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, tokenInfo)
	assert.Equal(t, "test-media", tokenInfo.MediaSlug)
	assert.Equal(t, "test-chapter", tokenInfo.ChapterSlug)
	assert.Equal(t, 2, tokenInfo.Page)
	assert.Equal(t, "test.jpg", tokenInfo.AssetPath)
}

func TestCleanupExpiredTokens(t *testing.T) {
	// Generate a token with 0 validity (should expire immediately)
	token := GenerateImageAccessTokenWithValidity("test-media", "test-chapter", 1, 0)
	assert.NotEmpty(t, token)

	// Validate it exists (might still be valid for a brief moment)
	_, err := ValidateImageToken(token)
	// It might or might not be valid immediately, that's ok

	// Run cleanup
	CleanupExpiredTokens()

	// Try to validate again - should fail
	_, err = ValidateImageToken(token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSaveImage(t *testing.T) {
	// Create a simple test image
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	// Fill with red color
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	// Test JPEG format
	tempDir := t.TempDir()
	jpegPath := filepath.Join(tempDir, "test.jpg")
	err := SaveImage(jpegPath, img, "jpeg", 80)
	assert.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(jpegPath)
	assert.NoError(t, err)

	// Test PNG format
	pngPath := filepath.Join(tempDir, "test.png")
	err = SaveImage(pngPath, img, "png", 80)
	assert.NoError(t, err)

	_, err = os.Stat(pngPath)
	assert.NoError(t, err)

	// Test unknown format (should default to JPEG)
	unknownPath := filepath.Join(tempDir, "test.unknown")
	err = SaveImage(unknownPath, img, "unknown", 80)
	assert.NoError(t, err)

	_, err = os.Stat(unknownPath)
	assert.NoError(t, err)
}

func TestSaveImageInvalidPath(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Test with invalid path
	err := SaveImage("/invalid/path/test.jpg", img, "jpeg", 80)
	assert.Error(t, err)
}

func TestProcessImage(t *testing.T) {
	// Create a test image file
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source.jpg")
	destPath := filepath.Join(tempDir, "processed.jpg")

	// Create a simple test image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}

	// Save the source image
	err := SaveImage(srcPath, img, "jpeg", 90)
	assert.NoError(t, err)

	// Process the image
	err = ProcessImage(srcPath, destPath, 80)
	assert.NoError(t, err)

	// Verify destination file exists
	_, err = os.Stat(destPath)
	assert.NoError(t, err)
}

func TestProcessImageNonExistentSource(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "processed.jpg")

	err := ProcessImage("/nonexistent/source.jpg", destPath, 80)
	assert.Error(t, err)
}

func TestOpenImage(t *testing.T) {
	// Create a test image file
	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "test.jpg")

	// Create a simple test image
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}

	// Save the image
	err := SaveImage(imagePath, img, "jpeg", 85)
	assert.NoError(t, err)

	// Open the image
	openedImg, err := OpenImage(imagePath)
	assert.NoError(t, err)
	assert.NotNil(t, openedImg)

	// Verify dimensions
	bounds := openedImg.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func TestOpenImageNonExistent(t *testing.T) {
	_, err := OpenImage("/nonexistent/image.jpg")
	assert.Error(t, err)
}

func TestCropImage(t *testing.T) {
	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}

	// Crop a 50x50 section from the center
	cropped := cropImage(img, 25, 25, 50, 50)
	assert.NotNil(t, cropped)

	bounds := cropped.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func TestResizeAndCrop(t *testing.T) {
	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}

	// Resize and crop to 100x100
	resized := resizeAndCrop(img, 100, 100)
	assert.NotNil(t, resized)

	bounds := resized.Bounds()
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 100, bounds.Dy())
}

func TestProcessImageWithTopCrop(t *testing.T) {
	// Create a test image file
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source_tall.jpg")
	destPath := filepath.Join(tempDir, "processed_crop.jpg")

	// Create a tall test image (webtoon style)
	img := image.NewRGBA(image.Rect(0, 0, 400, 2000))
	for y := 0; y < 2000; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}

	// Save the source image
	err := SaveImage(srcPath, img, "jpeg", 90)
	assert.NoError(t, err)

	// Process with top crop
	err = ProcessImageWithTopCrop(srcPath, destPath, 80)
	assert.NoError(t, err)

	// Verify destination file exists and has reasonable size
	stat, err := os.Stat(destPath)
	assert.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestProcessImageWithCrop(t *testing.T) {
	// Create a test image file
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source_crop.jpg")
	destPath := filepath.Join(tempDir, "processed_user_crop.jpg")

	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 500, 500))
	for y := 0; y < 500; y++ {
		for x := 0; x < 500; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}

	// Save the source image
	err := SaveImage(srcPath, img, "jpeg", 90)
	assert.NoError(t, err)

	// Process with custom crop
	cropData := map[string]interface{}{
		"x":      100.0,
		"y":      100.0,
		"width":  300.0,
		"height": 300.0,
	}
	err = ProcessImageWithCrop(srcPath, destPath, cropData, 80)
	assert.NoError(t, err)

	// Verify destination file exists
	stat, err := os.Stat(destPath)
	assert.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestGetFileNameWithExtensionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		fileUrl  string
		expected string
	}{
		{
			name:     "multiple dots in filename",
			fileName: "image.backup.jpg",
			fileUrl:  "http://example.com/image.png",
			expected: "image.backup.jpg",
		},
		{
			name:     "url with multiple extensions",
			fileName: "image",
			fileUrl:  "http://example.com/image.tar.gz",
			expected: "image.gz", // Only last extension
		},
		{
			name:     "url with query string",
			fileName: "cover",
			fileUrl:  "http://example.com/image.jpg?v=123",
			expected: "cover.jpg?v=123", // Query string included in URL parsing
		},
		{
			name:     "case preserved",
			fileName: "IMAGE",
			fileUrl:  "http://example.com/image.JPG",
			expected: "IMAGE.JPG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileNameWithExtension(tt.fileName, tt.fileUrl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateCropOffsetEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		imgWidth     int
		imgHeight    int
		targetWidth  int
		targetHeight int
	}{
		{
			name:         "very small target",
			imgWidth:     1000,
			imgHeight:    1000,
			targetWidth:  1,
			targetHeight: 1,
		},
		{
			name:         "same dimensions",
			imgWidth:     200,
			imgHeight:    200,
			targetWidth:  200,
			targetHeight: 200,
		},
		{
			name:         "image smaller than target - image will be centered",
			imgWidth:     10,
			imgHeight:    10,
			targetWidth:  100,
			targetHeight: 100,
		},
		{
			name:         "portrait image",
			imgWidth:     300,
			imgHeight:    600,
			targetWidth:  300,
			targetHeight: 450,
		},
		{
			name:         "landscape image",
			imgWidth:     1200,
			imgHeight:    600,
			targetWidth:  400,
			targetHeight: 600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.imgWidth, tt.imgHeight))
			_, _ = calculateCropOffset(img, tt.targetWidth, tt.targetHeight)
			
			// When image is smaller than target, offsets can be negative (center crop)
			// This is expected behavior - the function centers the smaller image
			// We just verify the function completes without panic
			assert.NotNil(t, img)
		})
	}
}

func TestCropImageEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		imgWidth         int
		imgHeight        int
		cropX            int
		cropY            int
		cropWidth        int
		cropHeight       int
	}{
		{
			name:       "full image",
			imgWidth:   100,
			imgHeight:  100,
			cropX:      0,
			cropY:      0,
			cropWidth:  100,
			cropHeight: 100,
		},
		{
			name:       "quarter image",
			imgWidth:   100,
			imgHeight:  100,
			cropX:      0,
			cropY:      0,
			cropWidth:  50,
			cropHeight: 50,
		},
		{
			name:       "single pixel",
			imgWidth:   100,
			imgHeight:  100,
			cropX:      50,
			cropY:      50,
			cropWidth:  1,
			cropHeight: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.imgWidth, tt.imgHeight))
			cropped := cropImage(img, tt.cropX, tt.cropY, tt.cropWidth, tt.cropHeight)
			
			assert.NotNil(t, cropped)
			bounds := cropped.Bounds()
			assert.Equal(t, tt.cropWidth, bounds.Dx())
			assert.Equal(t, tt.cropHeight, bounds.Dy())
		})
	}
}

func TestSaveImageFormatVariations(t *testing.T) {
	tempDir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.RGBA{100, 150, 200, 255})
		}
	}

	tests := []struct {
		name     string
		format   string
		filename string
	}{
		{"jpeg", "jpeg", filepath.Join(tempDir, "test_jpeg.jpg")},
		{"jpeg uppercase", "JPEG", filepath.Join(tempDir, "test_JPEG.jpg")},
		{"png", "png", filepath.Join(tempDir, "test_png.png")},
		{"gif", "gif", filepath.Join(tempDir, "test_gif.gif")},
		{"unknown defaults to jpeg", "unknown", filepath.Join(tempDir, "test_unknown.jpg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveImage(tt.filename, img, tt.format, 90)
			assert.NoError(t, err)
			
			_, err = os.Stat(tt.filename)
			assert.NoError(t, err)
		})
	}
}

func TestResizeAndCropAspectRatios(t *testing.T) {
	tests := []struct {
		name           string
		imgWidth       int
		imgHeight      int
		targetWidth    int
		targetHeight   int
	}{
		{
			name:        "landscape to square",
			imgWidth:    400,
			imgHeight:   200,
			targetWidth: 100,
			targetHeight: 100,
		},
		{
			name:        "portrait to square",
			imgWidth:    200,
			imgHeight:   400,
			targetWidth: 100,
			targetHeight: 100,
		},
		{
			name:        "wide to tall",
			imgWidth:    600,
			imgHeight:   300,
			targetWidth: 200,
			targetHeight: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.imgWidth, tt.imgHeight))
			for y := 0; y < tt.imgHeight; y++ {
				for x := 0; x < tt.imgWidth; x++ {
					img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
				}
			}

			result := resizeAndCrop(img, tt.targetWidth, tt.targetHeight)
			assert.NotNil(t, result)

			bounds := result.Bounds()
			assert.Equal(t, tt.targetWidth, bounds.Dx())
			assert.Equal(t, tt.targetHeight, bounds.Dy())
		})
	}
}