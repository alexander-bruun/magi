package metadata

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
)

// ComicInfo represents the ComicInfo.xml structure according to ComicInfo 2.1 schema
type ComicInfo struct {
	XMLName             xml.Name `xml:"ComicInfo"`
	Title               string   `xml:"Title,omitempty"`
	Series              string   `xml:"Series,omitempty"`
	Number              string   `xml:"Number,omitempty"`
	Count               int      `xml:"Count,omitempty"`
	Volume              int      `xml:"Volume,omitempty"`
	AlternateSeries     string   `xml:"AlternateSeries,omitempty"`
	AlternateNumber     string   `xml:"AlternateNumber,omitempty"`
	AlternateCount      int      `xml:"AlternateCount,omitempty"`
	Summary             string   `xml:"Summary,omitempty"`
	Notes               string   `xml:"Notes,omitempty"`
	Year                int      `xml:"Year,omitempty"`
	Month               int      `xml:"Month,omitempty"`
	Day                 int      `xml:"Day,omitempty"`
	Writer              string   `xml:"Writer,omitempty"`
	Penciller           string   `xml:"Penciller,omitempty"`
	Inker               string   `xml:"Inker,omitempty"`
	Colorist            string   `xml:"Colorist,omitempty"`
	Letterer            string   `xml:"Letterer,omitempty"`
	CoverArtist         string   `xml:"CoverArtist,omitempty"`
	Editor              string   `xml:"Editor,omitempty"`
	Translator          string   `xml:"Translator,omitempty"`
	Publisher           string   `xml:"Publisher,omitempty"`
	Imprint             string   `xml:"Imprint,omitempty"`
	Genre               string   `xml:"Genre,omitempty"`
	Tags                string   `xml:"Tags,omitempty"`
	Web                 string   `xml:"Web,omitempty"`
	PageCount           int      `xml:"PageCount,omitempty"`
	LanguageISO         string   `xml:"LanguageISO,omitempty"`
	Format              string   `xml:"Format,omitempty"`
	BlackAndWhite       string   `xml:"BlackAndWhite,omitempty"`
	Manga               string   `xml:"Manga,omitempty"`
	Characters          string   `xml:"Characters,omitempty"`
	Teams               string   `xml:"Teams,omitempty"`
	Locations           string   `xml:"Locations,omitempty"`
	ScanInformation     string   `xml:"ScanInformation,omitempty"`
	StoryArc            string   `xml:"StoryArc,omitempty"`
	StoryArcNumber      string   `xml:"StoryArcNumber,omitempty"`
	SeriesGroup         string   `xml:"SeriesGroup,omitempty"`
	AgeRating           string   `xml:"AgeRating,omitempty"`
	Pages               *Pages   `xml:"Pages,omitempty"`
	CommunityRating     float64  `xml:"CommunityRating,omitempty"`
	MainCharacterOrTeam string   `xml:"MainCharacterOrTeam,omitempty"`
	Review              string   `xml:"Review,omitempty"`
	GTIN                string   `xml:"GTIN,omitempty"`
}

// Pages represents the Pages element containing Page elements
type Pages struct {
	Page []Page `xml:"Page"`
}

// Page represents a single page in the comic
type Page struct {
	Image       int    `xml:"Image,attr"`
	Type        string `xml:"Type,attr,omitempty"`
	DoublePage  bool   `xml:"DoublePage,attr,omitempty"`
	ImageSize   int64  `xml:"ImageSize,attr,omitempty"`
	Key         string `xml:"Key,attr,omitempty"`
	Bookmark    string `xml:"Bookmark,attr,omitempty"`
	ImageWidth  int    `xml:"ImageWidth,attr,omitempty"`
	ImageHeight int    `xml:"ImageHeight,attr,omitempty"`
}

// GenerateComicInfo creates a ComicInfo struct from AggregatedMediaMetadata
func GenerateComicInfo(meta *AggregatedMediaMetadata, chapterNumber string, chapterTitle string, pageCount int) *ComicInfo {
	comicInfo := &ComicInfo{
		Title:       chapterTitle,
		Series:      meta.Title,
		Number:      chapterNumber,
		Count:       meta.ChapterCount,
		Volume:      meta.VolumeCount,
		Summary:     meta.Description,
		Year:        meta.Year,
		LanguageISO: meta.OriginalLanguage,
		PageCount:   pageCount,
		Tags:        strings.Join(meta.Tags, ", "),
		Genre:       strings.Join(meta.Genres, ", "),
		Characters:  strings.Join(meta.Characters, ", "),
		Publisher:   meta.Publisher,
		Imprint:     meta.Magazine,
		SeriesGroup: meta.Serialization,
		Format:      "Digital",
	}

	// Parse start date for month/day
	if meta.StartDate != "" {
		if year, month, day, err := parseDate(meta.StartDate); err == nil {
			if comicInfo.Year == 0 {
				comicInfo.Year = year
			}
			comicInfo.Month = month
			comicInfo.Day = day
		}
	}

	// Set Manga field based on type
	if meta.Type == "manga" {
		comicInfo.Manga = "YesAndRightToLeft"
	} else {
		comicInfo.Manga = "No"
	}

	// Set writers (authors)
	var writers []string
	for _, author := range meta.Authors {
		if author.Name != "" {
			writers = append(writers, author.Name)
		}
	}
	if len(writers) > 0 {
		comicInfo.Writer = strings.Join(writers, ", ")
	}

	// Set pencillers (artists)
	var pencillers []string
	for _, artist := range meta.Artists {
		if artist.Name != "" {
			pencillers = append(pencillers, artist.Name)
		}
	}
	if len(pencillers) > 0 {
		comicInfo.Penciller = strings.Join(pencillers, ", ")
	}

	// Set community rating
	if meta.AverageScore > 0 {
		comicInfo.CommunityRating = meta.AverageScore
	}

	// Set age rating based on content rating
	switch strings.ToLower(meta.ContentRating) {
	case "safe":
		comicInfo.AgeRating = "Everyone"
	case "suggestive":
		comicInfo.AgeRating = "Teen"
	case "erotica":
		comicInfo.AgeRating = "Mature 17+"
	case "pornographic":
		comicInfo.AgeRating = "Adults Only 18+"
	default:
		comicInfo.AgeRating = "Unknown"
	}

	// Set main character if available
	if len(meta.Characters) > 0 {
		comicInfo.MainCharacterOrTeam = meta.Characters[0]
	}

	// Add pages information if pageCount > 0
	if pageCount > 0 {
		pages := &Pages{}
		for i := 0; i < pageCount; i++ {
			pageType := "Story"
			if i == 0 {
				pageType = "FrontCover"
			}
			page := Page{
				Image: i,
				Type:  pageType,
			}
			pages.Page = append(pages.Page, page)
		}
		comicInfo.Pages = pages
	}

	return comicInfo
}

// GenerateComicInfoFromMedia creates a ComicInfo struct from a Media model
func GenerateComicInfoFromMedia(media *models.Media, chapterNumber string, chapterTitle string, pageCount int) *ComicInfo {
	// Convert Media to AggregatedMediaMetadata-like structure
	meta := &AggregatedMediaMetadata{
		Title:             media.Name,
		AlternativeTitles: media.AlternativeTitles,
		Description:       media.Description,
		Year:              media.Year,
		OriginalLanguage:  media.OriginalLanguage,
		Status:            media.Status,
		ContentRating:     media.ContentRating,
		Tags:              media.Tags,
		Type:              media.Type,
		Authors:           convertAuthorInfos(media.Authors),
		Artists:           convertAuthorInfos(media.Artists),
		StartDate:         media.StartDate,
		EndDate:           media.EndDate,
		ChapterCount:      media.ChapterCount,
		VolumeCount:       media.VolumeCount,
		AverageScore:      media.AverageScore,
		Popularity:        media.Popularity,
		Favorites:         media.Favorites,
		Demographic:       media.Demographic,
		Publisher:         media.Publisher,
		Magazine:          media.Magazine,
		Serialization:     media.Serialization,
		Genres:            media.Genres,
		Characters:        media.Characters,
	}

	return GenerateComicInfo(meta, chapterNumber, chapterTitle, pageCount)
}

// convertAuthorInfos converts []models.AuthorInfo to []AuthorInfo
func convertAuthorInfos(authors []models.AuthorInfo) []AuthorInfo {
	if authors == nil {
		return nil
	}
	result := make([]AuthorInfo, len(authors))
	for i, author := range authors {
		result[i] = AuthorInfo{
			Name: author.Name,
			Role: author.Role,
		}
	}
	return result
}

// ToXML marshals the ComicInfo to XML bytes
func (c *ComicInfo) ToXML() ([]byte, error) {
	return xml.MarshalIndent(c, "", "  ")
}

// parseDate parses a date string in YYYY-MM-DD format and returns year, month, day
func parseDate(dateStr string) (int, int, int, error) {
	parts := strings.Split(dateStr, "-")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid date format")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, err
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, err
	}

	return year, month, day, nil
}
