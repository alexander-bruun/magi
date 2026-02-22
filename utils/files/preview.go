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

// analyzeImageInterest scores an image for character artwork presence.
// The algorithm favors pages with colorful character art (skin tones, saturated
// colors, smooth gradients) and penalizes text-heavy pages (speech bubbles,
// bimodal black/white luminance). Higher scores = better character previews.
func analyzeImageInterest(img image.Image) float64 {
	// Scale down for fast processing
	small := resize.Resize(analysisScale, 0, img, resize.NearestNeighbor)
	bounds := small.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w < 3 || h < 3 {
		return 0
	}

	// Pre-allocate pixel data arrays
	totalPixels := float64(w * h)
	gray := make([][]float64, h)

	var totalR, totalG, totalB float64
	whitePixels := 0.0
	blackPixels := 0.0
	skinPixels := 0.0
	saturatedPixels := 0.0
	midTonePixels := 0.0
	var totalSaturation float64

	// Luminance histogram for bimodality detection (16 bins)
	const lumBins = 16
	var lumHist [lumBins]float64

	for y := 0; y < h; y++ {
		gray[y] = make([]float64, w)
		for x := 0; x < w; x++ {
			r, g, b, _ := small.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			rf, gf, bf := float64(r>>8), float64(g>>8), float64(b>>8)

			lum := 0.299*rf + 0.587*gf + 0.114*bf
			gray[y][x] = lum
			totalR += rf
			totalG += gf
			totalB += bf

			// Luminance histogram
			bin := int(lum / 256.0 * float64(lumBins))
			if bin >= lumBins {
				bin = lumBins - 1
			}
			lumHist[bin]++

			// Count extreme pixels
			if rf > 240 && gf > 240 && bf > 240 {
				whitePixels++
			}
			if rf < 15 && gf < 15 && bf < 15 {
				blackPixels++
			}

			// Count mid-tone pixels (characteristic of character shading/artwork)
			if lum > 40 && lum < 210 {
				midTonePixels++
			}

			// --- Color saturation (HSV-based) ---
			maxC := math.Max(rf, math.Max(gf, bf))
			minC := math.Min(rf, math.Min(gf, bf))
			chroma := maxC - minC
			sat := 0.0
			if maxC > 0 {
				sat = chroma / maxC
			}
			totalSaturation += sat
			if sat > 0.15 && maxC > 30 {
				saturatedPixels++
			}

			// --- Skin tone detection ---
			// Skin tones: warm hue, moderate saturation, not too dark/light
			if isSkinTone(rf, gf, bf, sat, maxC) {
				skinPixels++
			}
		}
	}

	// === SCORING ===

	// 1. Edge density (Sobel) — still useful but reduced weight
	edgeCount := 0.0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			gx := -gray[y-1][x-1] + gray[y-1][x+1] +
				-2*gray[y][x-1] + 2*gray[y][x+1] +
				-gray[y+1][x-1] + gray[y+1][x+1]
			gy := -gray[y-1][x-1] - 2*gray[y-1][x] - gray[y-1][x+1] +
				gray[y+1][x-1] + 2*gray[y+1][x] + gray[y+1][x+1]
			magnitude := math.Sqrt(gx*gx + gy*gy)
			if magnitude > edgeThreshold {
				edgeCount++
			}
		}
	}
	edgeDensity := edgeCount / totalPixels

	// 2. Color saturation score — characters have colored hair, skin, clothing
	avgSaturation := totalSaturation / totalPixels
	saturationRatio := saturatedPixels / totalPixels

	// 3. Skin tone ratio — strong signal for character presence
	skinRatio := skinPixels / totalPixels

	// 4. Mid-tone ratio — artwork has smooth gradients; text pages are bimodal (black+white)
	midToneRatio := midTonePixels / totalPixels

	// 5. Luminance bimodality penalty — detect text-heavy pages
	// Text pages have peaks at very low (ink) and very high (paper) luminance
	// with little in between. We measure this as a "text penalty".
	textPenalty := computeLuminanceBimodality(lumHist[:], totalPixels)

	// 6. White blob penalty — large contiguous white areas = speech bubbles
	whiteBlobPenalty := computeWhiteBlobPenalty(gray, w, h)

	// 7. Non-white ratio — still penalize mostly-blank pages
	nonWhiteRatio := 1.0 - (whitePixels / totalPixels)

	// 8. Spatial variance — detail spread across the image
	spatialScore := computeSpatialVariance(gray, w, h)

	// 9. Gradient smoothness — character shading produces smooth gradients,
	// while text has abrupt transitions. Measure ratio of soft edges to hard edges.
	gradientScore := computeGradientSmoothness(gray, w, h)

	// Combine with character-focused weights
	score := edgeDensity*15.0 + // Reduced: text also has high edges
		avgSaturation*25.0 + // Color saturation (characters are colorful)
		saturationRatio*20.0 + // Ratio of colorful pixels
		skinRatio*30.0 + // Skin tones = characters present
		midToneRatio*20.0 + // Mid-tones = artwork shading
		nonWhiteRatio*10.0 + // Penalize blank pages
		spatialScore*10.0 + // Content spread
		gradientScore*15.0 // Smooth gradients = artwork

	// Apply penalties
	score *= (1.0 - textPenalty*0.5)      // Reduce score for text-heavy pages
	score *= (1.0 - whiteBlobPenalty*0.4) // Reduce score for speech-bubble-heavy pages

	// Bonus: heavily penalize pages that are >60% black (solid/dark pages)
	blackRatio := blackPixels / totalPixels
	if blackRatio > 0.6 {
		score *= 0.3
	}

	return score
}

// isSkinTone detects whether an RGB pixel is likely a skin tone.
// Uses a broad range to cover light, medium, and dark skin across manga/webtoon art styles.
func isSkinTone(r, g, b, saturation, maxC float64) bool {
	// Too dark or too bright for skin
	if maxC < 40 || maxC > 250 {
		return false
	}
	// Skin needs some warmth: R should be dominant or close
	if r < g || r < b {
		return false
	}
	// Warm tone check: red-blue difference
	warmth := r - b
	if warmth < 10 {
		return false
	}
	// Saturation range for skin (not gray, not fully saturated)
	if saturation < 0.08 || saturation > 0.65 {
		return false
	}
	// Green should be between red and blue for natural skin
	if g < b {
		return false
	}
	// Additional constraint: luminance range typical for skin
	lum := 0.299*r + 0.587*g + 0.114*b
	if lum < 50 || lum > 230 {
		return false
	}
	return true
}

// computeLuminanceBimodality detects text-heavy pages by analyzing the luminance histogram.
// Text pages have strong peaks at the extremes (black ink + white paper) with a valley
// in the middle. Returns 0-1 where 1 = very bimodal (text-heavy).
func computeLuminanceBimodality(hist []float64, totalPixels float64) float64 {
	n := len(hist)
	if n < 4 {
		return 0
	}

	// Normalize histogram
	norm := make([]float64, n)
	for i, v := range hist {
		norm[i] = v / totalPixels
	}

	// Sum mass in bottom 3 bins (dark/ink) and top 3 bins (white/paper)
	darkMass := norm[0] + norm[1] + norm[2]
	lightMass := norm[n-1] + norm[n-2] + norm[n-3]

	// Sum mass in middle bins
	var middleMass float64
	for i := 3; i < n-3; i++ {
		middleMass += norm[i]
	}

	// Bimodality: strong extremes with weak middle
	extremeMass := darkMass + lightMass

	if extremeMass < 0.3 {
		return 0 // Not bimodal, plenty of mid-tones
	}

	// Ratio of extreme to middle — higher means more text-like
	if middleMass < 0.01 {
		return 1.0 // Almost entirely black + white
	}

	bimodality := extremeMass / (extremeMass + middleMass)
	// Scale to 0-1 range (only penalize when clearly bimodal)
	penalty := (bimodality - 0.5) * 2.0
	if penalty < 0 {
		penalty = 0
	}
	if penalty > 1 {
		penalty = 1
	}
	return penalty
}

// computeWhiteBlobPenalty estimates the presence of large white regions (speech bubbles).
// Divides the image into blocks and checks for clusters of mostly-white blocks.
// Returns 0-1 where 1 = many large white blobs.
func computeWhiteBlobPenalty(gray [][]float64, w, h int) float64 {
	blockSize := 8
	if w < blockSize*2 || h < blockSize*2 {
		return 0
	}

	blocksX := w / blockSize
	blocksY := h / blockSize
	totalBlocks := blocksX * blocksY
	if totalBlocks == 0 {
		return 0
	}

	whiteBlocks := 0
	for by := 0; by < blocksY; by++ {
		for bx := 0; bx < blocksX; bx++ {
			whiteCount := 0
			totalCount := 0
			for y := by * blockSize; y < (by+1)*blockSize && y < h; y++ {
				for x := bx * blockSize; x < (bx+1)*blockSize && x < w; x++ {
					totalCount++
					if gray[y][x] > 235 {
						whiteCount++
					}
				}
			}
			if totalCount > 0 && float64(whiteCount)/float64(totalCount) > 0.85 {
				whiteBlocks++
			}
		}
	}

	whiteBlockRatio := float64(whiteBlocks) / float64(totalBlocks)
	// Only penalize when >25% of blocks are white (significant speech bubbles)
	if whiteBlockRatio < 0.25 {
		return 0
	}
	// Scale: 25% → 0, 70% → 1
	penalty := (whiteBlockRatio - 0.25) / 0.45
	if penalty > 1 {
		penalty = 1
	}
	return penalty
}

// computeGradientSmoothness measures the ratio of soft gradients to hard edges.
// Character artwork has smooth shading gradients while text has abrupt black/white transitions.
// Returns 0-1 where higher = smoother gradients (more artwork-like).
func computeGradientSmoothness(gray [][]float64, w, h int) float64 {
	if w < 3 || h < 3 {
		return 0
	}

	softEdges := 0.0
	hardEdges := 0.0
	softThreshold := 15.0 // Subtle gradients
	hardThreshold := 60.0 // Sharp transitions (text edges)

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			gx := -gray[y-1][x-1] + gray[y-1][x+1] +
				-2*gray[y][x-1] + 2*gray[y][x+1] +
				-gray[y+1][x-1] + gray[y+1][x+1]
			gy := -gray[y-1][x-1] - 2*gray[y-1][x] - gray[y-1][x+1] +
				gray[y+1][x-1] + 2*gray[y+1][x] + gray[y+1][x+1]
			magnitude := math.Sqrt(gx*gx + gy*gy)

			if magnitude > softThreshold && magnitude < hardThreshold {
				softEdges++
			} else if magnitude >= hardThreshold {
				hardEdges++
			}
		}
	}

	totalEdges := softEdges + hardEdges
	if totalEdges == 0 {
		return 0
	}

	// Higher ratio of soft-to-hard edges = more artwork, less text
	return softEdges / totalEdges
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

// scoreRegion scores a horizontal strip for character artwork presence.
// Combines edge density, gradient smoothness, color saturation, and skin tones.
func scoreRegion(gray [][]float64, w, startY, endY int) float64 {
	if endY-startY < 3 || w < 3 {
		return 0
	}

	edgeCount := 0.0
	softEdges := 0.0
	hardEdges := 0.0
	var values []float64
	whitePixels := 0.0

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
			if mag > 15 && mag < 60 {
				softEdges++
			} else if mag >= 60 {
				hardEdges++
			}
			values = append(values, gray[y][x])
			if gray[y][x] > 235 {
				whitePixels++
			}
		}
	}

	totalPixels := float64(len(values))
	if totalPixels == 0 {
		return 0
	}

	// Edge density
	edgeDensity := edgeCount / totalPixels

	// Mid-tone ratio — artwork regions have lots of mid-luminance pixels
	midToneCount := 0.0
	for _, v := range values {
		if v > 40 && v < 210 {
			midToneCount++
		}
	}
	midToneRatio := midToneCount / totalPixels

	// Gradient smoothness — prefer soft shading over sharp text edges
	gradientScore := 0.0
	totalEdges := softEdges + hardEdges
	if totalEdges > 0 {
		gradientScore = softEdges / totalEdges
	}

	// White blob penalty — avoid regions dominated by speech bubbles
	whiteRatio := whitePixels / totalPixels
	whitePenalty := 0.0
	if whiteRatio > 0.3 {
		whitePenalty = (whiteRatio - 0.3) / 0.7
	}

	score := edgeDensity*20 +
		midToneRatio*35 +
		gradientScore*25 +
		(1.0-whitePenalty)*20

	return score
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
