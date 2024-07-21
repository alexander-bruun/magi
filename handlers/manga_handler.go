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
	slug := c.Params("slug")
	log.Info("????")

	id, err := models.GetMangaIDBySlug(slug)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	manga, err := models.GetManga(id)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	return HandleView(c, views.Manga(*manga))
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

	log.Info("1")
	mangaDetail, err := models.GetMangadexManga(mangadexID)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	log.Info("2")
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

	log.Info("3")
	log.Infof("Cover art url: %s", coverArtURL)
	u, err := url.Parse(coverArtURL)
	if err != nil {
		log.Errorf("Error parsing URL:", err)
	}

	log.Info("4")
	filename := filepath.Base(u.Path)
	fileExt := filepath.Ext(filename)
	fileExt = fileExt[1:]
	cachedImageURL := fmt.Sprintf("http://localhost:3000/api/images/%s.%s", slug, fileExt)

	log.Info("6")
	err = utils.DownloadImage("/home/alexa/magi/cache", slug, coverArtURL)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	log.Info("7")
	existingManga.Name = mangaName
	existingManga.Slug = slug
	existingManga.Description = mangaDetail.Attributes.Description["en"]
	existingManga.Year = mangaDetail.Attributes.Year
	existingManga.OriginalLanguage = mangaDetail.Attributes.OriginalLanguage
	existingManga.Status = mangaDetail.Attributes.Status
	existingManga.ContentRating = mangaDetail.Attributes.ContentRating
	existingManga.CoverArtURL = cachedImageURL

	log.Info("8")
	err = models.UpdateManga(existingManga)
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	log.Info("9")
	manga, err := models.GetManga(uint(mangaID))
	if err != nil {
		return HandleView(c, views.Error(err.Error()))
	}

	log.Info("10")
	return HandleView(c, views.Manga(*manga))
}
