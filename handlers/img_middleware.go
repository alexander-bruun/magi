package handlers

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

var (
	imgCache   = make(map[string]string) // path -> data URI
	imgCacheMu sync.RWMutex
)

// InitImgCache pre-loads image files as base64 data URIs
func InitImgCache(assetsPath string) error {
	imgDir := filepath.Join(assetsPath, "img")

	// For now, just cache icon.webp - expand as needed
	iconPath := filepath.Join(imgDir, "icon.webp")
	content, err := os.ReadFile(iconPath)
	if err != nil {
		return err
	}

	dataURI := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(content)

	imgCacheMu.Lock()
	imgCache["/assets/img/icon.webp"] = dataURI
	imgCacheMu.Unlock()

	log.Infof("Image cache initialized with %d files", len(imgCache))
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

		imgCacheMu.RLock()
		for urlPath, dataURI := range imgCache {
			html = strings.ReplaceAll(html, `"`+urlPath+`"`, `"`+dataURI+`"`)
		}
		imgCacheMu.RUnlock()

		c.Response().SetBody([]byte(html))
		return nil
	}
}
