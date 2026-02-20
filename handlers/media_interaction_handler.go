package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v3"
)

// MediaInteractionFormData represents form data for media interactions (vote/favorite)
type MediaInteractionFormData struct {
	Value string `json:"value"`
}

// MediaInteractionData holds data for media interactions like votes
type MediaInteractionData struct {
	Score     int
	UpVotes   int
	DownVotes int
	UserVote  int
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
func HandleMediaVote(c fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	var formData MediaInteractionFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	valStr := formData.Value
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	data, err := ProcessMediaVote(userName, mangaSlug, valStr)
	if err != nil {
		return SendInternalServerError(c, ErrMediaVoteFailed, err)
	}

	return handleView(c, views.MediaVoteFragment(mangaSlug, data.Score, data.UpVotes, data.DownVotes, data.UserVote))
}

// HandleMediaVoteFragment returns the vote UI fragment for a media. If user is logged in,
// it will show their current selection highlighted.
func HandleMediaVoteFragment(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the media page
	if !isHTMXRequest(c) {
		mangaSlug := c.Params("media")
		return c.Redirect().To("/series/" + mangaSlug)
	}

	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	score, up, down, err := models.GetMediaVotes(mangaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrMediaVoteFailed, err)
	}
	userVote := 0
	if userName != "" {
		v, _ := models.GetUserVoteForMedia(userName, mangaSlug)
		userVote = v
	}
	if userName == "" {
		// For anonymous users, show only the stats without buttons
		return handleView(c, views.MediaVoteFragmentAnonymous(mangaSlug, score, up, down))
	}
	return handleView(c, views.MediaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMediaFavorite handles a user's favorite/unfavorite action for a media via HTMX.
// Expected form values: "value" = "1" (favorite) or "0" (unfavorite). User must be authenticated.
func HandleMediaFavorite(c fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	var formData MediaInteractionFormData
	if err := c.Bind().Body(&formData); err != nil {
		return SendBadRequestError(c, ErrBadRequest)
	}

	valStr := formData.Value
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	if valStr == "0" {
		if err := models.RemoveFavorite(userName, mangaSlug); err != nil {
			return SendInternalServerError(c, ErrMediaFavoriteFailed, err)
		}
	} else {
		if err := models.SetFavorite(userName, mangaSlug); err != nil {
			return SendInternalServerError(c, ErrMediaFavoriteFailed, err)
		}
	}

	// Return updated fragment so HTMX can refresh the favorite UI in-place.
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrMediaFavoriteFailed, err)
	}
	isFav, _ := models.IsFavoriteForUser(userName, mangaSlug)
	return handleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleMediaFavoriteFragment returns the favorite UI fragment for a media.
func HandleMediaFavoriteFragment(c fiber.Ctx) error {
	// If not an HTMX request, redirect to the media page
	if !isHTMXRequest(c) {
		mangaSlug := c.Params("media")
		return c.Redirect().To("/series/" + mangaSlug)
	}

	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrMediaFavoriteFailed, err)
	}
	isFav := false
	if userName != "" {
		f, _ := models.IsFavoriteForUser(userName, mangaSlug)
		isFav = f
	}
	if userName == "" {
		// For anonymous users, show only the count without buttons
		return handleView(c, views.MediaFavoriteFragmentAnonymous(mangaSlug, favCount))
	}
	return handleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}

// HandleAddHighlight handles adding a media to highlights via HTMX.
// Uses the media's cover art as background and generates a default description.
func HandleAddHighlight(c fiber.Ctx) error {
	mediaSlug := c.Params("media")

	// Verify media exists
	media, err := models.GetMediaUnfiltered(mediaSlug)
	if err != nil || media == nil {
		return SendNotFoundError(c, ErrMediaNotFound)
	}

	// Check if already highlighted
	isHighlighted, err := models.IsMediaHighlighted(mediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if isHighlighted {
		triggerNotification(c, "This series is already highlighted", "warning")
		return c.SendString("")
	}

	// Use media cover art as background
	backgroundImageURL := media.CoverArtURL

	// Generate default description from media info
	description := fmt.Sprintf("%s - %s", media.Name, strings.ToUpper(media.Type))
	if media.Author != "" {
		description += fmt.Sprintf(" by %s", media.Author)
	}

	// Get next display order
	displayOrder := 0
	existingHighlights, err := models.GetHighlights()
	if err == nil {
		for _, h := range existingHighlights {
			if h.Highlight.DisplayOrder >= displayOrder {
				displayOrder = h.Highlight.DisplayOrder + 1
			}
		}
	}

	// Create highlight
	_, err = models.CreateHighlight(mediaSlug, backgroundImageURL, description, displayOrder)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerNotification(c, "Series added to highlights successfully", "success")
	return handleView(c, views.MediaHighlightFragment(mediaSlug, true))
}

// HandleRemoveHighlight handles removing a media from highlights via HTMX.
func HandleRemoveHighlight(c fiber.Ctx) error {
	mediaSlug := c.Params("media")

	// Verify media exists
	media, err := models.GetMediaUnfiltered(mediaSlug)
	if err != nil || media == nil {
		return SendNotFoundError(c, ErrMediaNotFound)
	}

	// Check if highlighted
	isHighlighted, err := models.IsMediaHighlighted(mediaSlug)
	if err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}
	if !isHighlighted {
		triggerNotification(c, "This series is not highlighted", "warning")
		return c.SendString("")
	}

	// Remove highlight
	if err := models.DeleteHighlightByMediaSlug(mediaSlug); err != nil {
		return SendInternalServerError(c, ErrInternalServerError, err)
	}

	triggerNotification(c, "Series removed from highlights successfully", "success")
	return handleView(c, views.MediaHighlightFragment(mediaSlug, false))
}
