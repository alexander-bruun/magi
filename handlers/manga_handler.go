package handlers

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

func HandleMangas(c *fiber.Ctx) error {
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page <= 0 {
		page = 1
	}

	mangas, count, err := models.SearchMangas("", page, 16, "name", "asc", "", "")
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Mangas(mangas, int(count), page))
}

func HandleManga(c *fiber.Ctx) error {
	slug := c.Params("manga")

	currentManga, err := models.GetManga(slug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapters, err := models.GetChapters(slug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Manga(*currentManga, chapters))
}

func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")

	manga, err := models.GetManga(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	currentSlug := chapter.Slug
	prevSlug, nextSlug, err := models.GetAdjacentChapters(currentSlug, mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapterFilePath := filepath.Join(manga.Path, chapter.File)
	pageCount, err := utils.CountImageFiles(chapterFilePath)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	var images []string
	for i := 1; i < pageCount; i++ {
		images = append(images, fmt.Sprintf("/api/comic?manga=%s&chapter=%s&page=%d", mangaSlug, chapterSlug, i))
	}

	return HandleView(c, views.Chapter(prevSlug, currentSlug, nextSlug, *manga, images, *chapter, chapters))
}

func HandleUpdateMetadataManga(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != err {
		return HandleView(c, views.Error(err.Error()))
	}
	search := c.Query("search")

	response, err := models.GetMangadexMangas(search)
	if err != err {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.UpdateMetadata(*response, uint(id)))
}

func HandleEditMetadataManga(c *fiber.Ctx) error {
	mangadexID := c.Query("mangadexid")
	mangaSlug := c.Query("manga")
	if mangaSlug == "" {
		return HandleView(c, views.Error("Manga slug can't be empty."))
	}

	// Fetch the existing manga
	existingManga, err := models.GetManga(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Fetch manga details from Mangadex
	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Extract manga details
	mangaName := mangaDetail.Attributes.Title["en"]
	var coverArtURL string

	// Extract cover art URL
	for _, rel := range mangaDetail.Relationships {
		if rel.Type == "cover_art" {
			if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
				if fileName, exists := attributes["fileName"].(string); exists {
					coverArtURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s",
						mangadexID,
						fileName)
					break // Assuming there's only one cover art
				}
			}
		}
	}

	// Handle local image if coverArtURL is empty
	if coverArtURL == "" {
		return HandleView(c, views.Error("Cover art URL not found."))
	}

	// Parse the cover art URL
	u, err := url.Parse(coverArtURL)
	if err != nil {
		log.Errorf("Error parsing URL: %v", err)
		return HandleView(c, views.Error("Error parsing cover art URL."))
	}

	// Extract file extension
	filename := filepath.Base(u.Path)
	fileExt := filepath.Ext(filename)
	fileExt = fileExt[1:] // remove leading dot

	// Create cached image URL
	cachedImageURL := fmt.Sprintf("http://localhost:3000/api/images/%s.%s", existingManga.Slug, fileExt)

	// Download and cache the image
	err = utils.DownloadImage("/home/alexa/magi/cache", existingManga.Slug, coverArtURL)
	if err != nil {
		log.Errorf("Error downloading image: %v", err)
		return HandleView(c, views.Error("Error downloading cover art image."))
	}

	// Update manga details
	existingManga.Name = mangaName
	existingManga.Description = mangaDetail.Attributes.Description["en"]
	existingManga.Year = mangaDetail.Attributes.Year
	existingManga.OriginalLanguage = mangaDetail.Attributes.OriginalLanguage
	existingManga.Status = mangaDetail.Attributes.Status
	existingManga.ContentRating = mangaDetail.Attributes.ContentRating
	existingManga.CoverArtURL = cachedImageURL

	// Save updated manga details
	err = models.UpdateManga(existingManga)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	// Redirect to updated manga page
	redirectURL := fmt.Sprintf("/mangas/%s", existingManga.Slug)
	c.Set("HX-Redirect", redirectURL)

	return c.SendStatus(fiber.StatusOK)
}

func HandleMangaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
	}

	mangas, _, err := models.SearchMangas(searchParam, 1, 10, "name", "desc", "", "")
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	if len(mangas) <= 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMangas(mangas))
}
