package utils

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// EPUB parsing structures (from POC)
type Chapter struct {
	ID    string
	Path  string
	Href  string
	Title string
}

type Container struct {
	XMLName    xml.Name `xml:"container"`
	Rootfiles  struct {
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
	idToHref := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		idToHref[item.ID] = item.Href
	}

	opfDir := filepath.Dir(opfPath)

	var chapters []Chapter
	for _, itemref := range pkg.Spine.Itemrefs {
		href, ok := idToHref[itemref.Idref]
		if !ok {
			continue
		}
		path := filepath.Join(opfDir, href)
		chapters = append(chapters, Chapter{ID: itemref.Idref, Path: path, Href: href})
	}

	return chapters, nil
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
	idToHref := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		idToHref[item.ID] = item.Href
	}

	validIds := make(map[string]bool)
	for _, itemref := range pkg.Spine.Itemrefs {
		href, ok := idToHref[itemref.Idref]
		if !ok {
			continue
		}
		chapterPath := filepath.Join(opfDir, href)
		validIds["chapter-" + chapterPath] = true
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

// GetBookContent extracts all content from an EPUB file as HTML
func GetBookContent(epubPath, lightNovelSlug, chapterSlug string) string {
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
		return "Error opening container.xml: " + err.Error()
	}
	defer rc.Close()
	containerData, err := io.ReadAll(rc)
	if err != nil {
		return "Error reading container.xml: " + err.Error()
	}

	var container Container
	err = xml.Unmarshal(containerData, &container)
	if err != nil {
		return "Error parsing container.xml: " + err.Error()
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
		return "Error opening OPF: " + err.Error()
	}
	defer rc2.Close()
	opfData, err := io.ReadAll(rc2)
	if err != nil {
		return "Error reading OPF: " + err.Error()
	}

	var pkg Package
	err = xml.Unmarshal(opfData, &pkg)
	if err != nil {
		return "Error parsing OPF: " + err.Error()
	}

	// Create map from id to href
	idToHref := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		idToHref[item.ID] = item.Href
	}

	opfDir := filepath.Dir(opfPath)

	// Read spine items
	var content strings.Builder
	for _, itemref := range pkg.Spine.Itemrefs {
		if itemref.Idref == "toc.xhtml" {
			continue // Skip the embedded TOC page since we have the sidebar
		}
		href, ok := idToHref[itemref.Idref]
		if !ok {
			continue
		}
		chapterPath := filepath.Join(opfDir, href)
		// Find the file
		var file *zip.File
		for _, f := range r.File {
			if f.Name == chapterPath {
				file = f
				break
			}
		}
		if file == nil {
			continue
		}
		frc, err := file.Open()
		if err != nil {
			continue
		}
		chapterData, err := io.ReadAll(frc)
		frc.Close()
		if err != nil {
			continue
		}
		chapterDir := filepath.Dir(chapterPath)
		chapterStr := string(chapterData)

		// Extract body content
		bodyStart := strings.Index(chapterStr, "<body")
		if bodyStart == -1 {
			continue
		}
		bodyStart = strings.Index(chapterStr[bodyStart:], ">") + bodyStart + 1
		bodyEnd := strings.LastIndex(chapterStr, "</body>")
		if bodyEnd == -1 {
			continue
		}
		chapterHTML := chapterStr[bodyStart:bodyEnd]

		// Replace src and href attributes to use asset endpoints
		chapterHTML = replaceAssetReferences(chapterHTML, chapterDir, lightNovelSlug, chapterSlug)

		// Add anchor div wrapping the chapter
		chapterHTML = fmt.Sprintf(`<div id="chapter-%s">`, chapterPath) + chapterHTML + `</div>`
		content.WriteString(chapterHTML)
	}

	if content.Len() == 0 {
		return "No content found in EPUB"
	}
	return content.String()
}

// replaceAssetReferences replaces src and href attributes in HTML to use asset endpoints
func replaceAssetReferences(html, chapterDir, lightNovelSlug, chapterSlug string) string {
	// Replace src="
	re := regexp.MustCompile(`src="([^"]*)"`)
	html = re.ReplaceAllStringFunc(html, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		src := sub[1]
		resolved := filepath.Join(chapterDir, src)
		return fmt.Sprintf(`src="/light-novels/%s/chapters/%s/%s"`, lightNovelSlug, chapterSlug, resolved)
	})

	// Replace src='
	re2 := regexp.MustCompile(`src='([^']*)'`)
	html = re2.ReplaceAllStringFunc(html, func(match string) string {
		sub := re2.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		src := sub[1]
		resolved := filepath.Join(chapterDir, src)
		return fmt.Sprintf(`src='/light-novels/%s/chapters/%s/%s'`, lightNovelSlug, chapterSlug, resolved)
	})

	// Replace href="
	re3 := regexp.MustCompile(`href="([^"]*)"`)
	html = re3.ReplaceAllStringFunc(html, func(match string) string {
		sub := re3.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		href := sub[1]
		resolved := filepath.Join(chapterDir, href)
		return fmt.Sprintf(`href="/light-novels/%s/chapters/%s/%s"`, lightNovelSlug, chapterSlug, resolved)
	})

	// Replace href='
	re4 := regexp.MustCompile(`href='([^']*)'`)
	html = re4.ReplaceAllStringFunc(html, func(match string) string {
		sub := re4.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		href := sub[1]
		resolved := filepath.Join(chapterDir, href)
		return fmt.Sprintf(`href='/light-novels/%s/chapters/%s/%s'`, lightNovelSlug, chapterSlug, resolved)
	})

	return html
}