//go:build !extended
// +build !extended

package handlers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nwaples/rardecode/v2"
)

// ProcessImageForServing processes an image for serving with PNG compression (fallback)
func ProcessImageForServing(filePath string, quality int) ([]byte, string, error) {
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
	// Fallback to PNG when WebP is not available
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, "", err
	}

	return buf.Bytes(), "image/png", nil
}

// ServeComicArchiveFromZIP serves an image from a ZIP archive with PNG fallback
func ServeComicArchiveFromZIP(filePath string, page int, quality int, disableWebpConversion bool) ([]byte, string, error) {
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

	if disableWebpConversion {
		// Serve original image without recompression
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, "", err
		}
		return data, getContentType(file.Name), nil
	}

	img, _, err := image.Decode(rc)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	// Fallback to PNG when WebP is not available
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

// ServeComicArchiveFromRAR serves an image from a RAR archive with PNG fallback
func ServeComicArchiveFromRAR(filePath string, page int, quality int, disableWebpConversion bool) ([]byte, string, error) {
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

	if disableWebpConversion {
		// Serve original image without recompression
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, "", err
		}
		return data, getContentType(imageFiles[page-1].Name), nil
	}

	img, _, err := image.Decode(r)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	// Fallback to PNG when WebP is not available
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
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
