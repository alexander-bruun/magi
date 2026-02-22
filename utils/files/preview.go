package files

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v3/log"
	"github.com/nfnt/resize"
	"github.com/nwaples/rardecode/v2"
	_ "golang.org/x/image/webp"
)

const (
	// Preview thumbnail dimensions (landscape/widescreen for chapter cards)
	previewWidth  = 320
	previewHeight = 180

	// Analysis parameters
	maxPagesToAnalyze = 10  // Scan up to this many pages to find a good preview
	analysisScale     = 100 // Scale images down to this width for fast analysis
	edgeThreshold     = 30  // Sobel edge threshold
	minVariance       = 200 // Minimum color variance to consider "interesting"
)

// ChapterPreviewResult holds the analysis score and page index
type ChapterPreviewResult struct {
	PageIndex int
	Score     float64
}

// GenerateChapterPreview analyzes a chapter's images and returns a JPEG preview thumbnail
// from the most visually interesting page. It uses edge detection, color variance, and
// content density to find pages where "something is happening" rather than blank/title pages.
func GenerateChapterPreview(chapterFilePath string) ([]byte, error) {
	img, err := FindBestPreviewImage(chapterFilePath)
	if err != nil {
		return nil, err
	}

	// Create the preview thumbnail with a landscape crop from the center
	thumb := createPreviewThumbnail(img, previewWidth, previewHeight)

	// Encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("failed to encode preview: %w", err)
	}

	return buf.Bytes(), nil
}

// FindBestPreviewImage analyzes pages and returns the decoded image of the best candidate.
func FindBestPreviewImage(chapterFilePath string) (image.Image, error) {
	fileInfo, err := os.Stat(chapterFilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat chapter path: %w", err)
	}

	if fileInfo.IsDir() {
		return findBestPreviewFromDirectory(chapterFilePath)
	}

	lowerPath := strings.ToLower(chapterFilePath)
	switch {
	case strings.HasSuffix(lowerPath, ".cbz"), strings.HasSuffix(lowerPath, ".zip"):
		return findBestPreviewFromZip(chapterFilePath)
	case strings.HasSuffix(lowerPath, ".cbr"), strings.HasSuffix(lowerPath, ".rar"):
		return findBestPreviewFromRar(chapterFilePath)
	default:
		// Single image file
		return OpenImage(chapterFilePath)
	}
}

// findBestPreviewFromDirectory scans a directory of images
func findBestPreviewFromDirectory(dirPath string) (image.Image, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var imageFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && isImageFile(entry.Name()) {
			imageFiles = append(imageFiles, filepath.Join(dirPath, entry.Name()))
		}
	}
	sort.Strings(imageFiles)

	if len(imageFiles) == 0 {
		return nil, fmt.Errorf("no images found in directory")
	}

	// Analyze a subset of pages (skip first page which is often a cover/title)
	candidates := selectCandidatePages(len(imageFiles))

	var bestScore float64
	bestIndex := candidates[0]

	for _, idx := range candidates {
		img, err := OpenImage(imageFiles[idx])
		if err != nil {
			continue
		}
		score := analyzeImageInterest(img)
		log.Debugf("Preview analysis: page %d score=%.2f", idx, score)
		if score > bestScore {
			bestScore = score
			bestIndex = idx
		}
	}

	return OpenImage(imageFiles[bestIndex])
}

// findBestPreviewFromZip scans a ZIP/CBZ archive
func findBestPreviewFromZip(zipPath string) (image.Image, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	// Collect sorted image file entries
	type zipEntry struct {
		name string
		file *zip.File
	}
	var imageEntries []zipEntry
	for _, f := range r.File {
		if isImageFile(f.Name) {
			imageEntries = append(imageEntries, zipEntry{f.Name, f})
		}
	}
	sort.Slice(imageEntries, func(i, j int) bool {
		return imageEntries[i].name < imageEntries[j].name
	})

	if len(imageEntries) == 0 {
		return nil, fmt.Errorf("no images in archive")
	}

	candidates := selectCandidatePages(len(imageEntries))

	var bestScore float64
	bestIndex := candidates[0]

	for _, idx := range candidates {
		img, err := decodeZipEntry(imageEntries[idx].file)
		if err != nil {
			continue
		}
		score := analyzeImageInterest(img)
		log.Debugf("Preview analysis (zip): page %d score=%.2f", idx, score)
		if score > bestScore {
			bestScore = score
			bestIndex = idx
		}
	}

	return decodeZipEntry(imageEntries[bestIndex].file)
}

// findBestPreviewFromRar scans a RAR/CBR archive
func findBestPreviewFromRar(rarPath string) (image.Image, error) {
	// First pass: count images and store names
	file1, err := os.Open(rarPath)
	if err != nil {
		return nil, err
	}
	reader1, err := rardecode.NewReader(file1)
	if err != nil {
		file1.Close()
		return nil, err
	}

	var imageNames []string
	for {
		header, err := reader1.Next()
		if err != nil {
			break
		}
		if isImageFile(header.Name) {
			imageNames = append(imageNames, header.Name)
		}
	}
	file1.Close()
	sort.Strings(imageNames)

	if len(imageNames) == 0 {
		return nil, fmt.Errorf("no images in RAR archive")
	}

	candidates := selectCandidatePages(len(imageNames))
	candidateSet := make(map[int]bool)
	for _, idx := range candidates {
		candidateSet[idx] = true
	}

	// Second pass: analyze candidate images
	file2, err := os.Open(rarPath)
	if err != nil {
		return nil, err
	}
	defer file2.Close()
	reader2, err := rardecode.NewReader(file2)
	if err != nil {
		return nil, err
	}

	var bestScore float64
	bestIndex := candidates[0]
	var bestImageData []byte
	currentIndex := 0

	for {
		header, err := reader2.Next()
		if err != nil {
			break
		}
		if !isImageFile(header.Name) {
			continue
		}

		if candidateSet[currentIndex] {
			data, err := io.ReadAll(reader2)
			if err != nil {
				currentIndex++
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				currentIndex++
				continue
			}
			score := analyzeImageInterest(img)
			log.Debugf("Preview analysis (rar): page %d score=%.2f", currentIndex, score)
			if score > bestScore {
				bestScore = score
				bestIndex = currentIndex
				bestImageData = data
			}
		}
		currentIndex++
	}

	if bestImageData == nil {
		return nil, fmt.Errorf("failed to extract any images from RAR")
	}
	_ = bestIndex

	img, _, err := image.Decode(bytes.NewReader(bestImageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode best image: %w", err)
	}
	return img, nil
}

// selectCandidatePages picks which page indices to analyze.
// Strategy: skip page 0 (often title/cover), sample pages from the first third of the chapter
// where action/content panels are most likely.
func selectCandidatePages(totalPages int) []int {
	if totalPages <= 1 {
		return []int{0}
	}
	if totalPages <= 3 {
		// For very short chapters, check all pages
		result := make([]int, totalPages)
		for i := range result {
			result[i] = i
		}
		return result
	}

	// Start from page 1 (skip cover/title page 0)
	// Sample evenly from the first 60% of pages (where content panels typically are)
	scanRange := int(math.Ceil(float64(totalPages) * 0.6))
	if scanRange < 3 {
		scanRange = 3
	}
	if scanRange > totalPages {
		scanRange = totalPages
	}

	numSamples := maxPagesToAnalyze
	if numSamples > scanRange-1 {
		numSamples = scanRange - 1
	}

	candidates := make([]int, 0, numSamples)
	step := float64(scanRange-1) / float64(numSamples)
	for i := 0; i < numSamples; i++ {
		idx := int(math.Round(float64(i)*step)) + 1 // +1 to skip page 0
		if idx >= totalPages {
			idx = totalPages - 1
		}
		// Avoid duplicates
		if len(candidates) == 0 || candidates[len(candidates)-1] != idx {
			candidates = append(candidates, idx)
		}
	}

	return candidates
}

// analyzeImageInterest scores an image based on how visually interesting it is.
// Higher scores mean more content, detail, and variation — good for previews.
// It combines: edge density, color variance, non-white content ratio, and contrast.
func analyzeImageInterest(img image.Image) float64 {
	// Scale down for fast processing
	small := resize.Resize(analysisScale, 0, img, resize.NearestNeighbor)
	bounds := small.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w < 3 || h < 3 {
		return 0
	}

	// Convert to grayscale array for edge detection
	gray := make([][]float64, h)
	var totalR, totalG, totalB float64
	var rValues, gValues, bValues []float64
	totalPixels := float64(w * h)
	whitePixels := 0.0
	blackPixels := 0.0

	for y := 0; y < h; y++ {
		gray[y] = make([]float64, w)
		for x := 0; x < w; x++ {
			r, g, b, _ := small.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			rf, gf, bf := float64(r>>8), float64(g>>8), float64(b>>8)

			gray[y][x] = 0.299*rf + 0.587*gf + 0.114*bf
			totalR += rf
			totalG += gf
			totalB += bf

			rValues = append(rValues, rf)
			gValues = append(gValues, gf)
			bValues = append(bValues, bf)

			// Count very white pixels (likely blank areas)
			if rf > 240 && gf > 240 && bf > 240 {
				whitePixels++
			}
			// Count very dark pixels (likely ink/outlines)
			if rf < 15 && gf < 15 && bf < 15 {
				blackPixels++
			}
		}
	}

	// 1. Edge density (Sobel operator) — measures detail and line art
	edgeCount := 0.0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			// Sobel X
			gx := -gray[y-1][x-1] + gray[y-1][x+1] +
				-2*gray[y][x-1] + 2*gray[y][x+1] +
				-gray[y+1][x-1] + gray[y+1][x+1]
			// Sobel Y
			gy := -gray[y-1][x-1] - 2*gray[y-1][x] - gray[y-1][x+1] +
				gray[y+1][x-1] + 2*gray[y+1][x] + gray[y+1][x+1]

			magnitude := math.Sqrt(gx*gx + gy*gy)
			if magnitude > edgeThreshold {
				edgeCount++
			}
		}
	}
	edgeDensity := edgeCount / totalPixels

	// 2. Color variance — measures visual richness
	meanR, meanG, meanB := totalR/totalPixels, totalG/totalPixels, totalB/totalPixels
	var varR, varG, varB float64
	for i := 0; i < len(rValues); i++ {
		varR += (rValues[i] - meanR) * (rValues[i] - meanR)
		varG += (gValues[i] - meanG) * (gValues[i] - meanG)
		varB += (bValues[i] - meanB) * (bValues[i] - meanB)
	}
	colorVariance := (varR + varG + varB) / (3 * totalPixels)

	// 3. Non-white ratio — penalize blank/mostly-white pages
	nonWhiteRatio := 1.0 - (whitePixels / totalPixels)

	// 4. Content balance — pages with a good mix of dark and light are more interesting
	// than uniformly colored pages
	blackRatio := blackPixels / totalPixels
	contentBalance := math.Min(blackRatio*10, 1.0) * math.Min(nonWhiteRatio*2, 1.0)

	// 5. Spatial variance — check if detail is spread across the image (not just one corner)
	spatialScore := computeSpatialVariance(gray, w, h)

	// Combine scores with weights
	score := edgeDensity*40.0 + // Edge detail is the strongest indicator
		(colorVariance/1000.0)*15.0 + // Color richness
		nonWhiteRatio*20.0 + // Penalize blank pages heavily
		contentBalance*15.0 + // Good balance of content
		spatialScore*10.0 // Content spread across the image

	return score
}

// computeSpatialVariance divides the image into a 3x3 grid and checks
// that edge content is distributed across multiple quadrants.
func computeSpatialVariance(gray [][]float64, w, h int) float64 {
	gridSize := 3
	cellW := w / gridSize
	cellH := h / gridSize

	if cellW < 2 || cellH < 2 {
		return 0
	}

	cellEdgeCounts := make([]float64, gridSize*gridSize)

	for gy := 0; gy < gridSize; gy++ {
		for gx := 0; gx < gridSize; gx++ {
			startX := gx * cellW
			startY := gy * cellH
			endX := startX + cellW
			endY := startY + cellH
			if endX > w-1 {
				endX = w - 1
			}
			if endY > h-1 {
				endY = h - 1
			}

			edgeCount := 0.0
			for y := startY + 1; y < endY-1; y++ {
				for x := startX + 1; x < endX-1; x++ {
					gxVal := -gray[y-1][x-1] + gray[y-1][x+1] +
						-2*gray[y][x-1] + 2*gray[y][x+1] +
						-gray[y+1][x-1] + gray[y+1][x+1]
					gyVal := -gray[y-1][x-1] - 2*gray[y-1][x] - gray[y-1][x+1] +
						gray[y+1][x-1] + 2*gray[y+1][x] + gray[y+1][x+1]
					mag := math.Sqrt(gxVal*gxVal + gyVal*gyVal)
					if mag > edgeThreshold {
						edgeCount++
					}
				}
			}
			cellEdgeCounts[gy*gridSize+gx] = edgeCount
		}
	}

	// Count how many cells have meaningful content
	activeCells := 0
	threshold := 5.0
	for _, count := range cellEdgeCounts {
		if count > threshold {
			activeCells++
		}
	}

	// Score: more active cells = better spatial distribution
	return float64(activeCells) / float64(gridSize*gridSize)
}

// createPreviewThumbnail creates a landscape-oriented preview crop from an image.
// For tall manga pages, it finds the most interesting vertical region to crop.
func createPreviewThumbnail(img image.Image, targetW, targetH int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// Target aspect ratio (landscape)
	targetAspect := float64(targetW) / float64(targetH)
	srcAspect := float64(srcW) / float64(srcH)

	var cropRect image.Rectangle

	if srcAspect >= targetAspect {
		// Image is wider than target — crop width, keep height
		cropH := srcH
		cropW := int(float64(cropH) * targetAspect)
		x := (srcW - cropW) / 2
		cropRect = image.Rect(x+bounds.Min.X, bounds.Min.Y, x+cropW+bounds.Min.X, cropH+bounds.Min.Y)
	} else {
		// Image is taller than target (typical manga page) — need to pick the best vertical region
		cropW := srcW
		cropH := int(float64(cropW) / targetAspect)
		if cropH > srcH {
			cropH = srcH
		}

		// Find the most interesting vertical strip
		bestY := findBestVerticalRegion(img, cropH)
		cropRect = image.Rect(bounds.Min.X, bestY+bounds.Min.Y, cropW+bounds.Min.X, bestY+cropH+bounds.Min.Y)
	}

	// Crop
	cropped := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(cropRect)

	// Resize to target dimensions
	return resize.Resize(uint(targetW), uint(targetH), cropped, resize.Lanczos3)
}

// findBestVerticalRegion slides a window down a tall image and scores each position
// to find the most visually dense/interesting region for the preview crop.
func findBestVerticalRegion(img image.Image, cropH int) int {
	bounds := img.Bounds()
	srcH := bounds.Dy()

	if cropH >= srcH {
		return 0
	}

	// Scale down for fast processing
	small := resize.Resize(uint(analysisScale), 0, img, resize.NearestNeighbor)
	smallBounds := small.Bounds()
	smallH := smallBounds.Dy()
	smallW := smallBounds.Dx()

	// Convert ratio
	scaleRatio := float64(smallH) / float64(srcH)
	smallCropH := int(float64(cropH) * scaleRatio)
	if smallCropH < 3 {
		smallCropH = 3
	}
	if smallCropH > smallH {
		smallCropH = smallH
	}

	// Convert to grayscale
	gray := make([][]float64, smallH)
	for y := 0; y < smallH; y++ {
		gray[y] = make([]float64, smallW)
		for x := 0; x < smallW; x++ {
			r, g, b, _ := small.At(x+smallBounds.Min.X, y+smallBounds.Min.Y).RGBA()
			gray[y][x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
		}
	}

	// Slide window and score each position
	step := int(math.Max(1, float64(smallH-smallCropH)/20))
	bestScore := -1.0
	bestY := 0

	for y := 0; y <= smallH-smallCropH; y += step {
		score := scoreRegion(gray, smallW, y, y+smallCropH)
		if score > bestScore {
			bestScore = score
			bestY = y
		}
	}

	// Convert back to original coordinates
	return int(float64(bestY) / scaleRatio)
}

// scoreRegion scores a horizontal strip of the grayscale image for edge density and contrast.
func scoreRegion(gray [][]float64, w, startY, endY int) float64 {
	if endY-startY < 3 || w < 3 {
		return 0
	}

	edgeCount := 0.0
	var values []float64

	for y := startY + 1; y < endY-1; y++ {
		for x := 1; x < w-1; x++ {
			gx := -gray[y-1][x-1] + gray[y-1][x+1] +
				-2*gray[y][x-1] + 2*gray[y][x+1] +
				-gray[y+1][x-1] + gray[y+1][x+1]
			gy := -gray[y-1][x-1] - 2*gray[y-1][x] - gray[y-1][x+1] +
				gray[y+1][x-1] + 2*gray[y+1][x] + gray[y+1][x+1]
			mag := math.Sqrt(gx*gx + gy*gy)
			if mag > edgeThreshold {
				edgeCount++
			}
			values = append(values, gray[y][x])
		}
	}

	totalPixels := float64(len(values))
	if totalPixels == 0 {
		return 0
	}

	// Edge density
	edgeDensity := edgeCount / totalPixels

	// Luminance variance (contrast)
	var mean float64
	for _, v := range values {
		mean += v
	}
	mean /= totalPixels
	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= totalPixels

	return edgeDensity*60 + (variance/1000)*40
}

// decodeZipEntry decodes an image from a zip file entry
func decodeZipEntry(f *zip.File) (image.Image, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	img, _, err := image.Decode(rc)
	return img, err
}

// GrayscaleImage is a simple grayscale image implementation
type GrayscaleImage struct {
	Pix    []uint8
	Width  int
	Height int
}

func (g *GrayscaleImage) ColorModel() color.Model { return color.GrayModel }
func (g *GrayscaleImage) Bounds() image.Rectangle { return image.Rect(0, 0, g.Width, g.Height) }
func (g *GrayscaleImage) At(x, y int) color.Color {
	return color.Gray{Y: g.Pix[y*g.Width+x]}
}
