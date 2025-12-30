package utils

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2/log"
)

// EPUB parsing structures (from POC)
type Chapter struct {
	ID    string
	Path  string
	Href  string
	Title string
}

type Container struct {
	XMLName   xml.Name `xml:"container"`
	Rootfiles struct {
		Rootfile struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfile"`
	} `xml:"rootfiles"`
}

type Package struct {
	XMLName  xml.Name `xml:"package"`
	Manifest struct {
		Items []struct {
			ID   string `xml:"id,attr"`
			Href string `xml:"href,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Itemrefs []struct {
			Idref string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

type NCX struct {
	XMLName xml.Name `xml:"ncx"`
	NavMap  struct {
		NavPoints []struct {
			NavLabel struct {
				Text string `xml:"text"`
			} `xml:"navLabel"`
			Content struct {
				Src string `xml:"src,attr"`
			} `xml:"content"`
		} `xml:"navPoint"`
	} `xml:"navMap"`
}

// GetChapters extracts chapter information from an EPUB file
func GetChapters(epubPath string) ([]Chapter, error) {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Find container.xml
	var containerFile *zip.File
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			containerFile = f
			break
		}
	}
	if containerFile == nil {
		return nil, fmt.Errorf("container.xml not found")
	}

	rc, err := containerFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	containerData, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var container Container
	err = xml.Unmarshal(containerData, &container)
	if err != nil {
		return nil, err
	}

	opfPath := container.Rootfiles.Rootfile.FullPath

	// Find OPF file
	var opfFile *zip.File
	for _, f := range r.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return nil, fmt.Errorf("OPF file not found: %s", opfPath)
	}

	rc2, err := opfFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc2.Close()
	opfData, err := io.ReadAll(rc2)
	if err != nil {
		return nil, err
	}

	var pkg Package
	err = xml.Unmarshal(opfData, &pkg)
	if err != nil {
		return nil, err
	}

	// Create map from id to href
	idToHref := sync.Map{} // id -> href
	for _, item := range pkg.Manifest.Items {
		idToHref.Store(item.ID, item.Href)
	}

	opfDir := filepath.Dir(opfPath)

	var chapters []Chapter
	for _, itemref := range pkg.Spine.Itemrefs {
		href, ok := idToHref.Load(itemref.Idref)
		if !ok {
			continue
		}
		path := filepath.Join(opfDir, href.(string))
		chapters = append(chapters, Chapter{ID: itemref.Idref, Path: path, Href: href.(string)})
	}

	return chapters, nil
}

// GetOPFDir returns the directory of the OPF file in the EPUB
func GetOPFDir(epubPath string) (string, error) {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	// Find container.xml
	var containerFile *zip.File
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			containerFile = f
			break
		}
	}
	if containerFile == nil {
		return "", fmt.Errorf("container.xml not found")
	}

	rc, err := containerFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	containerData, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	var container Container
	err = xml.Unmarshal(containerData, &container)
	if err != nil {
		return "", err
	}

	opfPath := container.Rootfiles.Rootfile.FullPath
	opfDir := filepath.Dir(opfPath)

	return opfDir, nil
}

// GetTitlesFromNCX extracts titles from NCX navigation
func GetTitlesFromNCX(epubPath string) (map[string]string, error) {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// Find container.xml
	var containerFile *zip.File
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			containerFile = f
			break
		}
	}
	if containerFile == nil {
		return nil, fmt.Errorf("container.xml not found")
	}

	rc, err := containerFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	containerData, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	var container Container
	err = xml.Unmarshal(containerData, &container)
	if err != nil {
		return nil, err
	}

	opfPath := container.Rootfiles.Rootfile.FullPath

	// Find OPF file
	var opfFile *zip.File
	for _, f := range r.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return nil, fmt.Errorf("OPF file not found: %s", opfPath)
	}

	rc2, err := opfFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc2.Close()
	opfData, err := io.ReadAll(rc2)
	if err != nil {
		return nil, err
	}

	var pkg Package
	err = xml.Unmarshal(opfData, &pkg)
	if err != nil {
		return nil, err
	}

	// Find NCX href
	var ncxHref string
	for _, item := range pkg.Manifest.Items {
		if strings.HasSuffix(item.Href, ".ncx") {
			ncxHref = item.Href
			break
		}
	}
	if ncxHref == "" {
		return nil, fmt.Errorf("NCX file not found")
	}

	opfDir := filepath.Dir(opfPath)
	ncxPath := filepath.Join(opfDir, ncxHref)

	// Find NCX file
	var ncxFile *zip.File
	for _, f := range r.File {
		if f.Name == ncxPath {
			ncxFile = f
			break
		}
	}
	if ncxFile == nil {
		return nil, fmt.Errorf("NCX file not found: %s", ncxPath)
	}

	rc3, err := ncxFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc3.Close()
	ncxData, err := io.ReadAll(rc3)
	if err != nil {
		return nil, err
	}

	var ncx NCX
	err = xml.Unmarshal(ncxData, &ncx)
	if err != nil {
		return nil, err
	}

	titleMap := make(map[string]string)
	for _, np := range ncx.NavMap.NavPoints {
		titleMap[np.Content.Src] = np.NavLabel.Text
	}

	return titleMap, nil
}

// ExtractTitle extracts title from HTML content
func ExtractTitle(html string) string {
	re := regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`)
	if match := re.FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	re2 := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	if match := re2.FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return "Untitled"
}

// GetTOC generates table of contents HTML from EPUB
func GetTOC(epubPath string) string {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return "Error opening EPUB: " + err.Error()
	}
	defer r.Close()

	// Find container.xml
	var containerFile *zip.File
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			containerFile = f
			break
		}
	}
	if containerFile == nil {
		return "container.xml not found"
	}

	rc, err := containerFile.Open()
	if err != nil {
		return err.Error()
	}
	defer rc.Close()
	containerData, err := io.ReadAll(rc)
	if err != nil {
		return err.Error()
	}

	var container Container
	err = xml.Unmarshal(containerData, &container)
	if err != nil {
		return err.Error()
	}

	opfPath := container.Rootfiles.Rootfile.FullPath

	// Find OPF file
	var opfFile *zip.File
	for _, f := range r.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}
	if opfFile == nil {
		return "OPF file not found: " + opfPath
	}

	rc2, err := opfFile.Open()
	if err != nil {
		return err.Error()
	}
	defer rc2.Close()
	opfData, err := io.ReadAll(rc2)
	if err != nil {
		return err.Error()
	}

	var pkg Package
	err = xml.Unmarshal(opfData, &pkg)
	if err != nil {
		return err.Error()
	}

	opfDir := filepath.Dir(opfPath)

	// Create map from id to href
	idToHref := sync.Map{} // id -> href
	for _, item := range pkg.Manifest.Items {
		idToHref.Store(item.ID, item.Href)
	}

	validIds := make(map[string]bool)
	for _, itemref := range pkg.Spine.Itemrefs {
		href, ok := idToHref.Load(itemref.Idref)
		if !ok {
			continue
		}
		chapterPath := filepath.Join(opfDir, href.(string))
		validIds["chapter-"+chapterPath] = true
	}

	// Find TOC href
	var tocHref string
	for _, item := range pkg.Manifest.Items {
		if strings.Contains(item.Href, "toc.") && strings.HasSuffix(item.Href, ".xhtml") {
			tocHref = item.Href
			break
		}
	}
	if tocHref == "" {
		return "TOC file not found"
	}

	tocPath := filepath.Join(opfDir, tocHref)
	tocDir := filepath.Dir(tocPath)

	// Find TOC file
	var tocFile *zip.File
	for _, f := range r.File {
		if f.Name == tocPath {
			tocFile = f
			break
		}
	}
	if tocFile == nil {
		return "TOC file not found: " + tocPath
	}

	rc3, err := tocFile.Open()
	if err != nil {
		return err.Error()
	}
	defer rc3.Close()
	tocData, err := io.ReadAll(rc3)
	if err != nil {
		return err.Error()
	}

	// Extract the nav epub:type="toc"
	tocHTML := string(tocData)
	start := strings.Index(tocHTML, `<nav epub:type="toc"`)
	if start == -1 {
		return "TOC nav not found"
	}
	end := strings.Index(tocHTML[start:], `</nav>`)
	if end == -1 {
		return "TOC nav end not found"
	}
	navContent := tocHTML[start : start+end+6]

	// Remove the <h1> title if present
	h1Start := strings.Index(navContent, "<h1")
	if h1Start != -1 {
		h1End := strings.Index(navContent[h1Start:], "</h1>")
		if h1End != -1 {
			h1End += h1Start + 5 // +5 for </h1>
			navContent = navContent[:h1Start] + navContent[h1End:]
		}
	}

	// Strip epub attributes to prevent layout issues
	navContent = regexp.MustCompile(`\s+epub:type="[^"]*"`).ReplaceAllString(navContent, "")
	navContent = regexp.MustCompile(`\s+id="[^"]*"`).ReplaceAllString(navContent, "")

	// Replace href="file" with href="#chapter-resolved-path"
	reHref := regexp.MustCompile(`href="([^"]*)"`)
	navContent = reHref.ReplaceAllStringFunc(navContent, func(match string) string {
		sub := reHref.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		href := sub[1]
		resolved := filepath.Join(tocDir, href)
		resolved = filepath.Clean(resolved)
		return fmt.Sprintf(`href="#chapter-%s"`, resolved)
	})

	// Remove li with invalid anchors
	reInvalid := regexp.MustCompile(`<li[^>]*><a href="#(chapter-[^"]*)">[^<]*</a></li>`)
	navContent = reInvalid.ReplaceAllStringFunc(navContent, func(match string) string {
		sub := reInvalid.FindStringSubmatch(match)
		if len(sub) > 1 {
			id := sub[1]
			if validIds[id] {
				return match
			}
		}
		return ""
	})

	// Add cover to TOC if not present
	var coverHref string
	for _, item := range pkg.Manifest.Items {
		if strings.Contains(strings.ToLower(item.ID), "cover") {
			coverHref = item.Href
			break
		}
	}
	if coverHref != "" {
		coverPath := filepath.Join(opfDir, coverHref)
		coverId := "chapter-" + coverPath
		if !strings.Contains(navContent, fmt.Sprintf(`href="#%s"`, coverId)) {
			olStart := strings.Index(navContent, "<ol")
			if olStart != -1 {
				olEnd := strings.Index(navContent[olStart:], ">") + olStart + 1
				navContent = navContent[:olEnd] + fmt.Sprintf(`<li><a href="#%s">Cover</a></li>`, coverId) + navContent[olEnd:]
			}
		}
	}

	return navContent
}

// GetBookContent extracts all readable content from an EPUB file as HTML
func GetBookContent(epubPath, mangaSlug, chapterSlug string) string {
	return GetBookContentWithValidity(epubPath, mangaSlug, chapterSlug, 5)
}

// GetBookContentWithValidity extracts all readable content from an EPUB file as HTML with custom token validity
func GetBookContentWithValidity(epubPath, mangaSlug, chapterSlug string, validityMinutes int) string {
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		return "Error opening EPUB: " + err.Error()
	}
	defer r.Close()

	chapters, err := GetChapters(epubPath)
	if err != nil {
		return "Error getting chapters: " + err.Error()
	}

	// Get OPF directory
	var opfPath string
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/content.opf") || f.Name == "content.opf" {
			opfPath = f.Name
			break
		}
	}
	if opfPath == "" {
		for _, f := range r.File {
			if f.Name == "META-INF/container.xml" {
				rc, err := f.Open()
				if err != nil {
					return "Error reading container: " + err.Error()
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return "Error reading container: " + err.Error()
				}
				var container Container
				err = xml.Unmarshal(data, &container)
				if err != nil {
					return "Error parsing container: " + err.Error()
				}
				opfPath = container.Rootfiles.Rootfile.FullPath
				break
			}
		}
	}
	opfDir := filepath.Dir(opfPath)

	var content strings.Builder
	for i, chapter := range chapters {
		// Skip table of contents chapters
		if strings.Contains(strings.ToLower(chapter.Path), "toc") ||
			strings.Contains(strings.ToLower(chapter.Href), "toc") {
			continue
		}

		// Find the chapter file
		var chapterFile *zip.File
		for _, f := range r.File {
			if f.Name == chapter.Path {
				chapterFile = f
				break
			}
		}
		if chapterFile == nil {
			continue
		}

		rc, err := chapterFile.Open()
		if err != nil {
			continue
		}
		chapterData, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		htmlContent := string(chapterData)
		// Clean the HTML and rewrite image paths
		htmlContent = cleanHTMLContentWithValidity(htmlContent, mangaSlug, chapterSlug, chapter.Path, opfDir, validityMinutes)

		// Add chapter ID
		content.WriteString(fmt.Sprintf(`<div id="chapter-%s">`, chapter.Path))
		content.WriteString(htmlContent)
		content.WriteString(`</div>`)

		// Add separator if not last
		if i < len(chapters)-1 {
			content.WriteString(`<hr>`)
		}
	}

	return content.String()
}

// cleanHTMLContent performs basic cleaning of HTML content from EPUB
func cleanHTMLContent(html, mangaSlug, chapterSlug, chapterPath, opfDir string) string {
	return cleanHTMLContentWithValidity(html, mangaSlug, chapterSlug, chapterPath, opfDir, 5)
}

// cleanHTMLContentWithValidity performs basic cleaning of HTML content from EPUB with custom token validity
func cleanHTMLContentWithValidity(html, mangaSlug, chapterSlug, chapterPath, opfDir string, validityMinutes int) string {
	// Remove DOCTYPE, html, head, body tags
	html = strings.ReplaceAll(html, "<!DOCTYPE html>", "")
	html = strings.ReplaceAll(html, "<html>", "")
	html = strings.ReplaceAll(html, "</html>", "")
	html = strings.ReplaceAll(html, "<head>", "")
	html = strings.ReplaceAll(html, "</head>", "")
	html = strings.ReplaceAll(html, "<body>", "")
	html = strings.ReplaceAll(html, "</body>", "")

	// Remove script and style tags
	for strings.Contains(html, "<script") {
		start := strings.Index(html, "<script")
		end := strings.Index(html[start:], "</script>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+9:]
	}

	for strings.Contains(html, "<style") {
		start := strings.Index(html, "<style")
		end := strings.Index(html[start:], "</style>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+8:]
	}

	// Remove link and meta tags
	for strings.Contains(html, "<link") {
		start := strings.Index(html, "<link")
		end := strings.Index(html[start:], ">")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+1:]
	}

	for strings.Contains(html, "<meta") {
		start := strings.Index(html, "<meta")
		end := strings.Index(html[start:], ">")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+1:]
	}

	// Rewrite img src attributes to point to asset endpoint
	html = rewriteAssetSourcesWithValidity(html, mangaSlug, chapterSlug, chapterPath, opfDir, validityMinutes)
	return html
}

// rewriteAssetSources rewrites img src and link href attributes to point to the asset endpoint with tokens
func rewriteAssetSources(html, mangaSlug, chapterSlug, chapterPath, opfDir string) string {
	return rewriteAssetSourcesWithValidity(html, mangaSlug, chapterSlug, chapterPath, opfDir, 5)
}

// rewriteAssetSourcesWithValidity rewrites img src and link href attributes to point to the asset endpoint with tokens and custom validity
func rewriteAssetSourcesWithValidity(html, mangaSlug, chapterSlug, chapterPath, opfDir string, validityMinutes int) string {
	// Use regex to find img tags with src attributes
	imgRe := regexp.MustCompile(`<img[^>]*src=(["']?)([^"'\s>]+)[^>]*>`)
	html = imgRe.ReplaceAllStringFunc(html, func(match string) string {
		// Extract the src value - find the position of src=
		srcIndex := strings.Index(match, `src=`)
		if srcIndex == -1 {
			return match
		}

		// Find the quote character
		quoteChar := ""
		valueStart := srcIndex + 4 // after src=
		if valueStart < len(match) {
			if match[valueStart] == '"' || match[valueStart] == '\'' {
				quoteChar = string(match[valueStart])
				valueStart++
			}
		}

		// Find the end of the value
		valueEnd := valueStart
		for valueEnd < len(match) {
			if quoteChar != "" {
				if match[valueEnd] == quoteChar[0] {
					break
				}
			} else {
				if match[valueEnd] == ' ' || match[valueEnd] == '>' || match[valueEnd] == '\t' {
					break
				}
			}
			valueEnd++
		}

		if valueStart >= valueEnd {
			return match
		}

		originalSrc := match[valueStart:valueEnd]

		// Skip if already an absolute URL or data URI
		if strings.HasPrefix(originalSrc, "http://") || strings.HasPrefix(originalSrc, "https://") || strings.HasPrefix(originalSrc, "data:") {
			return match
		}

		// Resolve the asset path relative to the chapter's directory, then relative to OPF dir
		chapterDir := filepath.Dir(chapterPath)
		absoluteAsset := filepath.Clean(filepath.Join(chapterDir, originalSrc))

		// Make it relative to the OPF directory
		var cleanPath string
		if strings.HasPrefix(absoluteAsset, opfDir+"/") {
			cleanPath = strings.TrimPrefix(absoluteAsset, opfDir+"/")
		} else if absoluteAsset == opfDir {
			cleanPath = ""
		} else {
			// If it's not under OPF dir, try the old cleaning method as fallback
			cleanPath = strings.ReplaceAll(filepath.Clean(originalSrc), "../", "")
			log.Warnf("Asset path %s resolved to %s which is not under OPF dir %s, using fallback cleaning: %s", originalSrc, absoluteAsset, opfDir, cleanPath)
		}

		// Generate a token for this asset
		token := GenerateImageAccessTokenWithAssetAndValidity(mangaSlug, chapterSlug, 0, cleanPath, validityMinutes) // Use page 0 for light novel assets
		log.Debugf("Generated token for light novel asset: media=%s, chapter=%s, original=%s, clean=%s, token=%s", mangaSlug, chapterSlug, originalSrc, cleanPath, token)

		// Build the asset URL with token
		assetURL := fmt.Sprintf("/api/image?token=%s", token)
		log.Debugf("originalSrc=%s, cleanPath=%s, token=%s\n", originalSrc, cleanPath, token)
		// Replace the src attribute
		oldAttr := `src=` + quoteChar + originalSrc + quoteChar
		newAttr := `src="` + assetURL + `"`
		log.Debugf("Replacing img src: %s -> %s", oldAttr, newAttr)
		return strings.Replace(match, oldAttr, newAttr, 1)
	})

	// Use regex to find link tags with href attributes
	linkRe := regexp.MustCompile(`<link[^>]*href=(["']?)([^"'\s>]+)[^>]*>`)
	html = linkRe.ReplaceAllStringFunc(html, func(match string) string {
		// Extract the href value - find the position of href=
		hrefIndex := strings.Index(match, `href=`)
		if hrefIndex == -1 {
			return match
		}

		// Find the quote character
		quoteChar := ""
		valueStart := hrefIndex + 5 // after href=
		if valueStart < len(match) {
			if match[valueStart] == '"' || match[valueStart] == '\'' {
				quoteChar = string(match[valueStart])
				valueStart++
			}
		}

		// Find the end of the value
		valueEnd := valueStart
		for valueEnd < len(match) {
			if quoteChar != "" {
				if match[valueEnd] == quoteChar[0] {
					break
				}
			} else {
				if match[valueEnd] == ' ' || match[valueEnd] == '>' || match[valueEnd] == '\t' {
					break
				}
			}
			valueEnd++
		}

		if valueStart >= valueEnd {
			return match
		}

		originalHref := match[valueStart:valueEnd]

		// Skip if already an absolute URL or data URI
		if strings.HasPrefix(originalHref, "http://") || strings.HasPrefix(originalHref, "https://") || strings.HasPrefix(originalHref, "data:") {
			return match
		}

		// Resolve the asset path relative to the chapter's directory, then relative to OPF dir
		chapterDir := filepath.Dir(chapterPath)
		absoluteAsset := filepath.Clean(filepath.Join(chapterDir, originalHref))

		// Make it relative to the OPF directory
		var cleanPath string
		if strings.HasPrefix(absoluteAsset, opfDir+"/") {
			cleanPath = strings.TrimPrefix(absoluteAsset, opfDir+"/")
		} else if absoluteAsset == opfDir {
			cleanPath = ""
		} else {
			// If it's not under OPF dir, try the old cleaning method as fallback
			cleanPath = strings.ReplaceAll(filepath.Clean(originalHref), "../", "")
			log.Warnf("Asset path %s resolved to %s which is not under OPF dir %s, using fallback cleaning: %s", originalHref, absoluteAsset, opfDir, cleanPath)
		}

		// Generate a token for this asset
		token := GenerateImageAccessTokenWithAssetAndValidity(mangaSlug, chapterSlug, 0, cleanPath, validityMinutes)
		log.Infof("Generated token for link asset %s: %s", cleanPath, token)

		// Build the asset URL with token
		assetURL := fmt.Sprintf("/api/image?token=%s", token)

		log.Infof("originalHref=%s, cleanPath=%s, token=%s\n", originalHref, cleanPath, token)

		// Replace the href attribute
		oldAttr := `href=` + quoteChar + originalHref + quoteChar
		newAttr := `href="` + assetURL + `"`
		return strings.Replace(match, oldAttr, newAttr, 1)
	})

	// Use regex to find a tags with href attributes
	aRe := regexp.MustCompile(`<a[^>]*href=(["']?)([^"'\s>]+)[^>]*>`)
	html = aRe.ReplaceAllStringFunc(html, func(match string) string {
		// Extract the href value - find the position of href=
		hrefIndex := strings.Index(match, `href=`)
		if hrefIndex == -1 {
			return match
		}

		// Find the quote character
		quoteChar := ""
		valueStart := hrefIndex + 5 // after href=
		if valueStart < len(match) {
			if match[valueStart] == '"' || match[valueStart] == '\'' {
				quoteChar = string(match[valueStart])
				valueStart++
			}
		}

		// Find the end of the value
		valueEnd := valueStart
		for valueEnd < len(match) {
			if quoteChar != "" {
				if match[valueEnd] == quoteChar[0] {
					break
				}
			} else {
				if match[valueEnd] == ' ' || match[valueEnd] == '>' || match[valueEnd] == '\t' {
					break
				}
			}
			valueEnd++
		}

		if valueStart >= valueEnd {
			return match
		}

		originalHref := match[valueStart:valueEnd]

		// If href starts with "/series/", disable the link to prevent navigation to wrong series
		if strings.HasPrefix(originalHref, "/series/") {
			oldAttr := `href=` + quoteChar + originalHref + quoteChar
			newAttr := `href="#"`
			return strings.Replace(match, oldAttr, newAttr, 1)
		}

		return match
	})

	return html
}
