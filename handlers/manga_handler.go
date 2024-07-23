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

	mangas, count, err := models.SearchMangas("", page, 16, "name", "asc", "", 0)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Mangas(mangas, int(count), page))
}

func HandleManga(c *fiber.Ctx) error {
	manga := c.Params("manga")

	id, err := models.GetMangaIDBySlug(manga)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	currentManga, err := models.GetManga(id)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapters, err := models.GetChapters(id)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Manga(*currentManga, chapters))
}

func HandleChapter(c *fiber.Ctx) error {
	mangaSlug := c.Params("manga")
	chapterSlug := c.Params("chapter")

	mangaID, err := models.GetMangaIDBySlug(mangaSlug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapterID, err := models.GetChapterIDBySlug(chapterSlug, mangaID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	manga, err := models.GetManga(mangaID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapter, err := models.GetChapter(chapterID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	currentSlug := chapter.Slug
	prevSlug, nextSlug, err := models.GetAdjacentChapters(currentSlug, mangaID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	chapterFilePath := filepath.Join(manga.Path, chapter.File)
	pageCount, err := utils.CountImageFilesInZip(chapterFilePath)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	var images []string
	for i := 1; i < pageCount; i++ {
		images = append(images, fmt.Sprintf("/api/comic?manga=%s&chapter=%s&page=%d", mangaSlug, chapterSlug, i))
	}

	return HandleView(c, views.Chapter(prevSlug, currentSlug, nextSlug, *manga, images, *chapter))
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
	mangaID, err := strconv.ParseUint(c.Query("mangaid"), 10, 64)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	existingManga, err := models.GetManga(uint(mangaID))
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	mangaName := mangaDetail.Attributes.Title["en"]
	slug := utils.Sluggify(mangaName)
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
	cachedImageURL := fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt)

	err = utils.DownloadImage("/home/alexa/magi/cache", slug, coverArtURL)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	existingManga.Name = mangaName
	existingManga.Slug = slug
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

	redirectURL := fmt.Sprintf("/%s", slug)
	c.Set("HX-Redirect", redirectURL)

	return c.SendStatus(fiber.StatusOK)
}

func HandleMangaSearch(c *fiber.Ctx) error {
	searchParam := c.Query("search")

	if searchParam == "" {
		return HandleView(c, views.OneDoesNotSimplySearch())
	}

	mangas, _, err := models.SearchMangas(searchParam, 1, 10, "name", "desc", "", 0)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	if len(mangas) <= 0 {
		return HandleView(c, views.NoResultsSearch())
	}

	return HandleView(c, views.SearchMangas(mangas))
}
