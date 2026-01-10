//go:build extended
// +build extended

package files

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/chai2010/webp"
)

// EncodeImageToBytes encodes an image to bytes in the specified format
func EncodeImageToBytes(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		// Ensure quality is at least 1 for JPEG encoding (Go's jpeg.Encode requires 1-100)
		jpegQuality := quality
		if jpegQuality < 1 {
			jpegQuality = 1
		}
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return nil, err
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	case "gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return nil, err
		}
	case "webp":
		// WebP quality is 0-100, lossy
		webpQuality := float32(quality)
		if webpQuality < 0 {
			webpQuality = 0
		}
		if webpQuality > 100 {
			webpQuality = 100
		}
		if err := webp.Encode(&buf, img, &webp.Options{Quality: webpQuality}); err != nil {
			return nil, err
		}
	default:
		// Unknown format - save as WebP
		webpQuality := float32(quality)
		if webpQuality < 0 {
			webpQuality = 0
		}
		if webpQuality > 100 {
			webpQuality = 100
		}
		if err := webp.Encode(&buf, img, &webp.Options{Quality: webpQuality}); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
