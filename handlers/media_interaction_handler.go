package handlers

import (
	"strconv"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	fiber "github.com/gofiber/fiber/v2"
)

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

	valStr := c.FormValue("value")
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	data, err := ProcessMediaVote(userName, mangaSlug, valStr)
	if err != nil {
		return handleError(c, err)
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
		return handleError(c, err)
	}
	userVote := 0
	if userName != "" {
		v, _ := models.GetUserVoteForMedia(userName, mangaSlug)
		userVote = v
	}
	return HandleView(c, views.MediaVoteFragment(mangaSlug, score, up, down, userVote))
}

// HandleMediaFavorite handles toggling a favorite for the logged-in user via HTMX.
// Expected form values: "value" = "1" to favorite or "0" to unfavorite.
func HandleMediaFavorite(c *fiber.Ctx) error {
	mangaSlug := c.Params("media")
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	valStr := c.FormValue("value")
	if valStr == "" {
		return fiber.ErrBadRequest
	}

	if valStr == "0" {
		if err := models.RemoveFavorite(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	} else {
		if err := models.SetFavorite(userName, mangaSlug); err != nil {
			return handleError(c, err)
		}
	}

	// Return updated fragment so HTMX can refresh the favorite UI in-place.
	favCount, err := models.GetFavoritesCount(mangaSlug)
	if err != nil {
		return handleError(c, err)
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
		return handleError(c, err)
	}
	isFav := false
	if userName != "" {
		f, _ := models.IsFavoriteForUser(userName, mangaSlug)
		isFav = f
	}
	return HandleView(c, views.MediaFavoriteFragment(mangaSlug, favCount, isFav))
}