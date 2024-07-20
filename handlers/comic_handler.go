package handlers

import (
	"archive/zip"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2"
	"github.com/nwaples/rardecode"
)

func ComicHandler(c *fiber.Ctx) error {
	// Query parameters for extracting comic images from chapters and pages
	manga := c.Query("comic")
	chapter := c.Query("chapter")
	page := c.Query("page")

	if manga != "" || chapter == "" || page == "" {
		return c.Status(fiber.StatusBadRequest).SendString("When requesting a comic, all parameters must be provided.")
	}

	filePath := fmt.Sprintf("%s/%s", "/mnt/l/somedir", manga) // TO-DO: Use library paths.

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("File or directory not found")
	}

	// Serve the file based on its extension
	switch {
	case strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".jpg"), strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".png"):
		return c.SendFile(filePath)
	case strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".rar"):
		return serveComicBookArchiveFromRAR(c, filePath)
	case strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".cbz"):
		return serveComicBookArchiveFromCBZ(c, filePath)
	default:
		return c.Status(fiber.StatusUnsupportedMediaType).SendString("Unsupported file type")
	}
}

// Serve images from RAR archives using rardecode
func serveComicBookArchiveFromRAR(c *fiber.Ctx, filePath string) error {
	// Open the RAR archive
	rarFile, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open archive")
	}
	defer rarFile.Close()

	// Create a new RAR reader
	rarReader, err := rardecode.NewReader(rarFile, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create RAR reader")
	}

	// Read through the archive entries
	for {
		header, err := rarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to read archive entry")
		}

		// Check if the entry is a file and ends with .jpg or .png
		if !header.IsDir && (strings.HasSuffix(strings.ToLower(header.Name), ".jpg") || strings.HasSuffix(strings.ToLower(header.Name), ".png")) {
			// Set Content-Type header based on file extension
			contentType := "image/jpeg" // Default to JPEG
			if strings.HasSuffix(strings.ToLower(header.Name), ".png") {
				contentType = "image/png"
			}
			c.Set("Content-Type", contentType)

			// Copy the image data to the response writer
			if _, err := io.Copy(c.Response().BodyWriter(), rarReader); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to write image to response")
			}
			return nil
		}
	}

	// If no image was found in the archive
	return c.Status(fiber.StatusNotFound).SendString("No images found in archive")
}

// Serve images from CBZ archives using archive/zip
func serveComicBookArchiveFromCBZ(c *fiber.Ctx, filePath string) error {
	// Get the page parameter from the query string
	pageStr := c.Query("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	// Open the CBZ archive
	cbzFile, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open archive")
	}
	defer cbzFile.Close()

	// Create a new ZIP reader
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create ZIP reader")
	}
	defer zipReader.Close()

	// Filter out only image files
	var imageFiles []*zip.File
	for _, file := range zipReader.File {
		if !file.FileInfo().IsDir() && (strings.HasSuffix(strings.ToLower(file.Name), ".jpg") || strings.HasSuffix(strings.ToLower(file.Name), ".png")) {
			imageFiles = append(imageFiles, file)
		}
	}

	// Check if the page number is within the range of available images
	if page > len(imageFiles) {
		return c.Status(fiber.StatusBadRequest).SendString("Page number out of range")
	}

	// Get the specified image file
	imageFile := imageFiles[page-1]

	// Set Content-Type header based on file extension
	contentType := "image/jpeg" // Default to JPEG
	if strings.HasSuffix(strings.ToLower(imageFile.Name), ".png") {
		contentType = "image/png"
	}
	c.Set("Content-Type", contentType)

	// Open the file from the ZIP archive
	rc, err := imageFile.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read image from archive")
	}
	defer rc.Close()

	// Copy the image data to the response writer
	if _, err := io.Copy(c.Response().BodyWriter(), rc); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to write image to response")
	}

	return nil
}

// Serve directory listing for one layer of directories
func serveDirectoryListing(c *fiber.Ctx, dirPath string, urlPath string) error {
	// Read directory entries
	fileInfos, err := os.ReadDir(dirPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to read directory %s: %v", dirPath, err))
	}

	// Prepare FileInfo for current directory
	currentDir := &models.FileInfo{
		Name:  filepath.Base(dirPath),
		IsDir: true,
	}

	// Iterate over directory entries
	for _, fileInfo := range fileInfos {
		// Skip hidden files and directories
		if strings.HasPrefix(fileInfo.Name(), ".") {
			continue
		}

		// Create FileInfo for current item
		itemInfo := &models.FileInfo{
			Name:  fileInfo.Name(),
			IsDir: fileInfo.IsDir(),
		}

		// Add itemInfo to currentDir's children
		currentDir.Children = append(currentDir.Children, itemInfo)
	}

	// Render directory listing template
	return renderDirectoryListing(c, currentDir, urlPath)
}

// Render directory listing using a template
func renderDirectoryListing(c *fiber.Ctx, dirInfo *models.FileInfo, urlPath string) error {
	// Define template for directory listing
	const tmpl = `
	<!DOCTYPE html>
	<html>
	<head>
			<meta charset="UTF-8">
			<title>Index of {{ .Name }}</title>
			<style>
					body { font-family: Arial, sans-serif; }
					ul { list-style-type: none; padding-left: 0; }
					li { margin-bottom: 5px; }
					a { text-decoration: none; color: blue; }
					a:hover { text-decoration: underline; }
			</style>
	</head>
	<body>
			<h1>Index of {{ .Name }}</h1>
			{{ if .Parent }}
			<a href="{{ .Parent }}">../</a>
			{{ end }}
			<ul>
					{{ range .Children }}
							<li>
									<a href="{{ pathJoin .Name }}">{{ .Name }}</a> 
							</li>
					{{ end }}
			</ul>
	</body>
	</html>
	`

	// Create template function map with pathJoin function
	funcMap := template.FuncMap{
		"pathJoin": func(name string) string {
			// Ensure urlPath ends with a trailing slash
			if !strings.HasSuffix(urlPath, "/") {
				urlPath += "/"
			}
			// Join the current URL path (urlPath) with the name of the file or directory
			// This ensures that clicking on a file maintains the correct path structure
			return urlPath + strings.TrimPrefix(name, "/")
		},
	}

	// Determine parent directory path
	parentPath := path.Dir(urlPath)
	if parentPath == "." {
		parentPath = "/"
	}

	// Add parent directory to dirInfo if not root
	if urlPath != "/" {
		dirInfo.Parent = parentPath
	}

	// Create and execute template with function map
	t := template.Must(template.New("directory").Funcs(funcMap).Parse(tmpl))
	err := t.Execute(c.Response().BodyWriter(), dirInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render directory listing")
	}

	return nil
}

// isAllowedPath checks if the requestedPath starts with any of the allowedPaths
func isAllowedPath(requestedPath string, allowedPaths []string) bool {
	for _, allowedPath := range allowedPaths {
		if strings.HasPrefix(requestedPath, allowedPath) {
			return true
		}
	}
	return false
}
