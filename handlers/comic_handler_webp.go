//go:build extended
// +build extended

package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chai2010/webp"
	"github.com/nwaples/rardecode/v2"
)

// ProcessImageForServing processes an image for serving with WebP compression
func ProcessImageForServing(filePath string) ([]byte, string, error) {
	// Check if the file is already WebP
	if strings.ToLower(filepath.Ext(filePath)) == ".webp" {
		// Serve WebP as is
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, "", err
		}
		return data, "image/webp", nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	// Use WebP at full quality
	err = webp.Encode(&buf, img, &webp.Options{Quality: 100})
	if err != nil {
		return nil, "", err
	}

	return buf.Bytes(), "image/webp", nil
}

// ServeComicArchiveFromZIP serves an image from a ZIP archive with WebP encoding
func ServeComicArchiveFromZIP(filePath string, page int) ([]byte, string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()

	// Get sorted list of image files
	var imageFiles []string
	for _, f := range r.File {
		lowerName := strings.ToLower(f.Name)
		if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
			strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".gif") ||
			strings.HasSuffix(lowerName, ".webp") {
			imageFiles = append(imageFiles, f.Name)
		}
	}

	sort.Strings(imageFiles)

	if page < 1 || page > len(imageFiles) {
		return nil, "", fmt.Errorf("page %d out of range", page)
	}

	file := r.File[page-1]
	rc, err := file.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()

	// Check if the file is already WebP
	if strings.ToLower(filepath.Ext(file.Name)) == ".webp" {
		// Serve WebP as is
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, "", err
		}
		return data, "image/webp", nil
	}

	img, _, err := image.Decode(rc)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	// Use WebP at full quality
	if err := webp.Encode(&buf, img, &webp.Options{Quality: 100}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/webp", nil
}

// ServeComicArchiveFromRAR serves an image from a RAR archive with WebP encoding
func ServeComicArchiveFromRAR(filePath string, page int) ([]byte, string, error) {
	r, err := rardecode.OpenReader(filePath)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()

	// Get sorted list of image files
	var imageFiles []*rardecode.FileHeader
	for {
		header, err := r.Next()
		if err != nil {
			break
		}
		lowerName := strings.ToLower(header.Name)
		if strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") ||
			strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".gif") ||
			strings.HasSuffix(lowerName, ".webp") {
			imageFiles = append(imageFiles, header)
		}
	}

	sort.Slice(imageFiles, func(i, j int) bool {
		return imageFiles[i].Name < imageFiles[j].Name
	})

	if page < 1 || page > len(imageFiles) {
		return nil, "", fmt.Errorf("page %d out of range", page)
	}

	// Skip to the desired file
	r, err = rardecode.OpenReader(filePath)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()

	for i := 0; i < page; i++ {
		_, err := r.Next()
		if err != nil {
			return nil, "", err
		}
	}

	// Serve raw image bytes without processing
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	contentType := getContentType(imageFiles[page-1].Name)
	return data, contentType, nil
}

// getContentType returns the content type for a file extension
func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
