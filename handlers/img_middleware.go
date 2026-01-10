package handlers

import (
	"encoding/base64"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

var (
	imgCache sync.Map // string -> []byte, concurrent
)

// InitImgCache pre-loads image files as base64 data URIs
func InitImgCache(assetsFS fs.FS, assetsPath string) error {
	imgDir := filepath.Join(assetsPath, "img")

	// For now, just cache icon.webp - expand as needed
	iconPath := filepath.Join(imgDir, "icon.webp")
	content, err := fs.ReadFile(assetsFS, iconPath)
	if err != nil {
		return err
	}

	dataURI := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(content)

	imgCache.Store("/assets/img/icon.webp", dataURI)

	// Count entries in sync.Map
	count := 0
	imgCache.Range(func(_, _ any) bool {
		count++
		return true
	})
	log.Infof("Image cache initialized with %d files", count)
	return nil
}

// ImgMiddleware replaces image src/href with inline data URIs
func ImgMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip non-HTML responses
		path := c.Path()
		if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/api/") {
			return c.Next()
		}

		err := c.Next()
		if err != nil {
			return err
		}

		contentType := string(c.Response().Header.ContentType())
		if !strings.Contains(contentType, "text/html") {
			return nil
		}

		body := c.Response().Body()
		if len(body) == 0 {
			return nil
		}

		html := string(body)

		imgCache.Range(func(key, value any) bool {
			urlPath := key.(string)
			dataURI := value.(string)
			html = strings.ReplaceAll(html, `"`+urlPath+`"`, `"`+dataURI+`"`)
			return true
		})

		c.Response().SetBody([]byte(html))
		return nil
	}
}
