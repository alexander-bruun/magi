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
)

const (
	defaultPage     = 1
	defaultPageSize = 16
	searchPageSize  = 10
)

func HandleMangas(c *fiber.Ctx) error {
	page := getPageNumber(c.Query("page"))
	mangas, count, err := models.SearchMangas("", page, defaultPageSize, "name", "asc", "", "")
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Mangas(mangas, int(count), page))
}

func HandleManga(c *fiber.Ctx) error {
	slug := c.Params("manga")
	manga, err := models.GetManga(slug)
	if err != nil {
		return handleError(c, err)
	}
	chapters, err := models.GetChapters(slug)
	if err != nil {
		return handleError(c, err)
	}
	return HandleView(c, views.Manga(*manga, chapters))
}

func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")

	manga, chapters, err := getMangaAndChapters(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}

	chapter, err := models.GetChapter(mangaSlug, chapterSlug)
	if err != nil {
		return handleError(c, err)
	}

	prevSlug, nextSlug, err := models.GetAdjacentChapters(chapter.Slug, mangaSlug)
	if err != nil {
		return handleError(c, err)
	}

	images, err := getChapterImages(manga, chapter)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.Chapter(prevSlug, chapter.Slug, nextSlug, *manga, images, *chapter, chapters))
}

func HandleUpdateMetadataManga(c *fiber.Ctx) error {
	mangaSlug := c.Params("slug")
	search := c.Query("search")

	response, err := models.GetMangadexMangas(search)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.UpdateMetadata(*response, mangaSlug))
}

func HandleEditMetadataManga(c *fiber.Ctx) error {
	mangadexID := c.Query("id")
	mangaSlug := c.Query("slug")
	if mangaSlug == "" {
		return handleError(c, fmt.Errorf("manga slug can't be empty"))
	}

	existingManga, err := models.GetManga(mangaSlug)
	if err != nil {
		return handleError(c, err)
	}

	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return handleError(c, err)
	}

	coverArtURL, err := extractCoverArtURL(mangaDetail, mangadexID)
	if err != nil {
		return handleError(c, err)
	}

	cachedImageURL, err := cacheAndGetImageURL(existingManga.Slug, coverArtURL)
	if err != nil {
		return handleError(c, err)
	}

	updateMangaDetails(existingManga, mangaDetail, cachedImageURL)

	if err := models.UpdateManga(existingManga); err != nil {
		return handleError(c, err)
	}

	redirectURL := fmt.Sprintf("/mangas/%s", existingManga.Slug)
	c.Set("HX-Redirect", redirectURL)
	return c.SendStatus(fiber.StatusOK)
}

func HandleMangaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
	}

	mangas, _, err := models.SearchMangas(searchParam, defaultPage, searchPageSize, "name", "desc", "", "")
	if err != nil {
		return handleError(c, err)
	}

	if len(mangas) == 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMangas(mangas))
}

// Helper functions

func getPageNumber(pageStr string) int {
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		return defaultPage
	}
	return page
}

func getMangaAndChapters(mangaSlug string) (*models.Manga, []models.Chapter, error) {
	manga, err := models.GetManga(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	chapters, err := models.GetChapters(mangaSlug)
	if err != nil {
		return nil, nil, err
	}

	return manga, chapters, nil
}

func getChapterImages(manga *models.Manga, chapter *models.Chapter) ([]string, error) {
	chapterFilePath := filepath.Join(manga.Path, chapter.File)
	pageCount, err := utils.CountImageFiles(chapterFilePath)
	if err != nil {
		return nil, err
	}

	images := make([]string, pageCount-1)
	for i := range images {
		images[i] = fmt.Sprintf("/api/comic?manga=%s&chapter=%s&page=%d", manga.Slug, chapter.Slug, i+1)
	}

	return images, nil
}

func extractCoverArtURL(mangaDetail *models.MangaDetail, mangadexID string) (string, error) {
	for _, rel := range mangaDetail.Relationships {
		if rel.Type == "cover_art" {
			if attributes, ok := rel.Attributes.(map[string]interface{}); ok {
				if fileName, exists := attributes["fileName"].(string); exists {
					return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", mangadexID, fileName), nil
				}
			}
		}
	}
	return "", fmt.Errorf("cover art URL not found")
}

func cacheAndGetImageURL(slug, coverArtURL string) (string, error) {
	u, err := url.Parse(coverArtURL)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}

	filename := filepath.Base(u.Path)
	fileExt := filepath.Ext(filename)[1:] // remove leading dot

	err = utils.DownloadImage("/home/alexa/magi/cache", slug, coverArtURL)
	if err != nil {
		return "", fmt.Errorf("error downloading image: %w", err)
	}

	return fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt), nil
}

func updateMangaDetails(manga *models.Manga, mangaDetail *models.MangaDetail, coverArtURL string) {
	manga.Name = mangaDetail.Attributes.Title["en"]
	manga.Description = mangaDetail.Attributes.Description["en"]
	manga.Year = mangaDetail.Attributes.Year
	manga.OriginalLanguage = mangaDetail.Attributes.OriginalLanguage
	manga.Status = mangaDetail.Attributes.Status
	manga.ContentRating = mangaDetail.Attributes.ContentRating
	manga.CoverArtURL = coverArtURL
}
