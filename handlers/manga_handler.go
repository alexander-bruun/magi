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

	existingManga, err := models.GetManga(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	mangaName := mangaDetail.Attributes.Title["en"]
	coverArtURL := ""
	for _, rel := range mangaDetail.Relationships {
		if rel.Type == "cover_art" {
			coverArtURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s",
				mangadexID,
				rel.Attributes.FileName)
		}
	}

	u, err := url.Parse(coverArtURL)
	if err != nil {
		log.Errorf("Error parsing URL:", err)
	}

	filename := filepath.Base(u.Path)
	fileExt := filepath.Ext(filename)
	fileExt = fileExt[1:]
	cachedImageURL := fmt.Sprintf("http://localhost:3000/api/images/%s.%s", existingManga.Slug, fileExt)

	err = utils.DownloadImage("/home/alexa/magi/cache", existingManga.Slug, coverArtURL)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	existingManga.Name = mangaName
	existingManga.Description = mangaDetail.Attributes.Description["en"]
	existingManga.Year = mangaDetail.Attributes.Year
	existingManga.OriginalLanguage = mangaDetail.Attributes.OriginalLanguage
	existingManga.Status = mangaDetail.Attributes.Status
	existingManga.ContentRating = mangaDetail.Attributes.ContentRating
	existingManga.CoverArtURL = cachedImageURL

	err = models.UpdateManga(existingManga)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
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

	mangas, _, err := models.SearchMangas(searchParam, 1, 10, "name", "desc", "", "")
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	if len(mangas) <= 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMangas(mangas))
}
