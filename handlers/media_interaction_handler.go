package handlers

import (
	"fmt"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/utils"
	"github.com/alexander-bruun/magi/views"
	"github.com/nfnt/resize"
	fiber "github.com/gofiber/fiber/v2"
)

// MediaInteractionFormData represents form data for media interactions (vote/favorite)
type MediaInteractionFormData struct {
	Value string `json:"value"`
}

// MediaInteractionData holds data for media interactions like votes
type MediaInteractionData struct {
	Score    int
	UpVotes  int
	DownVotes int
	UserVote int
}

// ProcessMediaVote processes a vote for media
func ProcessMediaVote(userName, mediaSlug, valueStr string) (*MediaInteractionData, error) {
	v, err := strconv.Atoi(valueStr)
	if err != nil {
		return nil, err
	}

	// If value == 0, remove vote
	if v == 0 {
		if err := models.RemoveVote(userName, mediaSlug); err != nil {
			return nil, err
		}
	} else {
		if err := models.SetVote(userName, mediaSlug, v); err != nil {
			return nil, err
		}
	}

	// Return updated vote data
	score, up, down, err := models.GetMediaVotes(mediaSlug)
	if err != nil {
		return nil, err
	}
	userVote, _ := models.GetUserVoteForMedia(userName, mediaSlug)

	return &MediaInteractionData{
		Score:     score,
		UpVotes:   up,
		DownVotes: down,
		UserVote:  userVote,
	}, nil
}

// ToggleMediaFavorite toggles favorite status for a media
func ToggleMediaFavorite(userName, mediaSlug string) error {
	return models.ToggleFavorite(userName, mediaSlug)
}

// GetMediaInteractionData gets interaction data for a media
func GetMediaInteractionData(mediaSlug, userName string) (*MediaInteractionData, error) {
	score, up, down, err := models.GetMediaVotes(mediaSlug)
	if err != nil {
		return nil, err
	}
	userVote, _ := models.GetUserVoteForMedia(userName, mediaSlug)

	return &MediaInteractionData{
		Score:     score,
		UpVotes:   up,
		DownVotes: down,
		UserVote:  userVote,
	}, nil
}

// HandleMediaVote handles a user's upvote/downvote for a media via HTMX.
// Expected form values: "value" = "1" or "-1". User must be authenticated.
func HandleMediaVote(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	var formData MediaInteractionFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	valStr := formData.Value
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	data, err := ProcessMediaVote(userName, mangaSlug, valStr)
	if err != nil {
		return sendInternalServerError(c, ErrMediaVoteFailed, err)
	}

	return HandleView(c, views.MediaVoteFragment(mangaSlug, data.Score, data.UpVotes, data.DownVotes, data.UserVote))
}

// HandleMediaVoteFragment returns the vote UI fragment for a media. If user is logged in,
// it will show their current selection highlighted.
func HandleMediaVoteFragment(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the media page
	if !IsHTMXRequest(c) {
		mangaSlug := c.Params("media")
		return c.Redirect("/series/" + mangaSlug)
	}

	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	score, up, down, err := models.GetMediaVotes(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrMediaVoteFailed, err)
	}
	userVote := 0
	if userName != "" {
		v, _ := models.GetUserVoteForMedia(userName, mangaSlug)
		userVote = v
	}
	return HandleView(c, views.MediaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMediaFavorite handles a user's favorite/unfavorite action for a media via HTMX.
// Expected form values: "value" = "1" (favorite) or "0" (unfavorite). User must be authenticated.
func HandleMediaFavorite(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	var formData MediaInteractionFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	valStr := formData.Value
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	if valStr == "0" {
		if err := models.RemoveFavorite(userName, mangaSlug); err != nil {
			return sendInternalServerError(c, ErrMediaFavoriteFailed, err)
		}
	} else {
		if err := models.SetFavorite(userName, mangaSlug); err != nil {
			return sendInternalServerError(c, ErrMediaFavoriteFailed, err)
		}
	}

	// Return updated fragment so HTMX can refresh the favorite UI in-place.
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrMediaFavoriteFailed, err)
	}
	isFav, _ := models.IsFavoriteForUser(userName, mangaSlug)
	return HandleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleMediaFavoriteFragment returns the favorite UI fragment for a media.
func HandleMediaFavoriteFragment(c *fiber.Ctx) error {
	// If not an HTMX request, redirect to the media page
	if !IsHTMXRequest(c) {
		mangaSlug := c.Params("media")
		return c.Redirect("/series/" + mangaSlug)
	}

	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrMediaFavoriteFailed, err)
	}
	isFav := false
	if userName != "" {
		f, _ := models.IsFavoriteForUser(userName, mangaSlug)
		isFav = f
	}
	return HandleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleAddHighlight handles adding a media to highlights via HTMX modal form.
// Expected form values: "background_image_url", "background_image" (file), "description", "display_order"
func HandleAddHighlight(c *fiber.Ctx) error {
	if cacheManager == nil {
		return sendInternalServerError(c, ErrInternalServerError, fmt.Errorf("cache not initialized"))
	}

	mediaSlug := c.Params("media")

	// Verify media exists
	media, err := models.GetMediaUnfiltered(mediaSlug)
	if err != nil || media == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Check if already highlighted
	isHighlighted, err := models.IsMediaHighlighted(mediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if isHighlighted {
		triggerNotification(c, "This series is already highlighted", "warning")
		return c.SendString("")
	}

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	// Get form values
	description := ""
	if desc := form.Value["description"]; len(desc) > 0 {
		description = desc[0]
	}
	displayOrderStr := ""
	if order := form.Value["display_order"]; len(order) > 0 {
		displayOrderStr = order[0]
	}
	backgroundImageURL := ""
	if url := form.Value["background_image_url"]; len(url) > 0 {
		backgroundImageURL = url[0]
	}

	displayOrder := 0
	if displayOrderStr != "" {
		if order, err := strconv.Atoi(displayOrderStr); err == nil {
			displayOrder = order
		}
	}

	// Handle image
	var finalImageURL string
	cacheDir := utils.GetCacheDirectory()
	imagesDir := filepath.Join(cacheDir, "images")

	// Check for uploaded file
	if files := form.File["background_image"]; len(files) > 0 {
		file := files[0]
		// Handle upload
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		// Save uploaded file temporarily
		tempPath := filepath.Join(imagesDir, fmt.Sprintf("temp_highlight_%s_%d", mediaSlug, time.Now().Unix()))
		if err := c.SaveFile(file, tempPath); err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		defer os.Remove(tempPath) // Clean up temp file

	// Load and convert to JPG
	img, err := utils.OpenImage(tempPath)
	if err != nil {
		triggerNotification(c, "Invalid image file: "+err.Error(), "destructive")
		return c.SendString("")
	}

	// Resize to banner dimensions (1200x400)
	resizedImg := resize.Resize(1200, 400, img, resize.Lanczos3)

	imageData, err := utils.EncodeImageToBytes(resizedImg, "jpeg", models.GetProcessedImageQuality())
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	cachePath := fmt.Sprintf("images/highlights_%s.jpg", mediaSlug)
	if err := cacheManager.Save(cachePath, imageData); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	finalImageURL = fmt.Sprintf("/api/images/highlights_%s.jpg", mediaSlug)
	} else if backgroundImageURL != "" {
		// Download from URL
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}

		// Download and resize to banner dimensions
		req, err := http.NewRequest("GET", backgroundImageURL, nil)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		req.Header.Set("User-Agent", "Magi/1.0")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return sendInternalServerError(c, ErrInternalServerError, fmt.Errorf("failed to download image: %s", resp.Status))
		}
	downloadedImg, _, err := image.Decode(resp.Body)
	if err != nil {
		triggerNotification(c, "Invalid image from URL: "+err.Error(), "destructive")
		return c.SendString("")
	}

	// Resize to banner dimensions (1200x400)
	bannerImg := resize.Resize(1200, 400, downloadedImg, resize.Lanczos3)

	imageData, err := utils.EncodeImageToBytes(bannerImg, "jpeg", models.GetProcessedImageQuality())
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	cachePath := fmt.Sprintf("images/highlights_%s.jpg", mediaSlug)
	if err := cacheManager.Save(cachePath, imageData); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	finalImageURL = fmt.Sprintf("/api/images/highlights_%s.jpg", mediaSlug)
	} else {
		return c.Status(fiber.StatusBadRequest).SendString("Either upload an image or provide a URL")
	}

	// Create highlight
	_, err = models.CreateHighlight(mediaSlug, finalImageURL, description, displayOrder)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerNotification(c, "Series added to highlights successfully", "success")
	return c.SendString(`<script>UIkit.modal('#add-highlight-modal').hide();</script>`)
}

// HandleRemoveHighlight handles removing a media from highlights via HTMX.
func HandleRemoveHighlight(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")

	// Verify media exists
	media, err := models.GetMediaUnfiltered(mediaSlug)
	if err != nil || media == nil {
		return sendNotFoundError(c, ErrMediaNotFound)
	}

	// Check if highlighted
	isHighlighted, err := models.IsMediaHighlighted(mediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if !isHighlighted {
		triggerNotification(c, "This series is not highlighted", "warning")
		return c.SendString("")
	}

	// Remove highlight
	if err := models.DeleteHighlightByMediaSlug(mediaSlug); err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerNotification(c, "Series removed from highlights successfully", "success")
	// Return the "Add to Highlights" button HTML
	return HandleView(c, templ.Raw(`
		<button
			type="button"
			class="uk-btn uk-btn-success uk-btn-small"
			uk-toggle="target: #add-highlight-modal"
			title="Add to highlights"
			aria-label="Add to highlights"
		>
			<uk-icon icon="Star"></uk-icon>
			<span class="hidden md:inline ml-1">Add to Highlights</span>
		</button>
	`))
}