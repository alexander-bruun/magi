package handlers

import (
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// HandleCollections displays all collections
func HandleCollections(c *fiber.Ctx) error {
	collections, err := models.GetAllCollections()
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.Collections(collections))
}

// HandleUserCollections displays collections created by the current user
func HandleUserCollections(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	collections, err := models.GetCollectionsByUser(userName)
	if err != nil {
		return handleError(c, err)
	}

	return HandleView(c, views.Collections(collections))
}

// HandleCollection displays a specific collection
func HandleCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil {
		return c.Status(404).SendString("Collection not found")
	}

	media, err := models.GetCollectionMedia(id)
	if err != nil {
		return handleError(c, err)
	}

	collectionWithMedia := models.CollectionWithMedia{
		Collection: *collection,
		Media:      media,
	}

	userName := GetUserContext(c)
	canEdit := userName != "" && (userName == collection.CreatedBy || userName == "admin" || userName == "moderator")

	return HandleView(c, views.Collection(collectionWithMedia, canEdit))
}

// HandleCreateCollectionForm displays the create collection form
func HandleCreateCollectionForm(c *fiber.Ctx) error {
	return HandleView(c, views.CreateCollection())
}

// HandleCreateCollection processes collection creation
func HandleCreateCollection(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))

	if name == "" {
		return c.Status(400).SendString("Collection name is required")
	}

	collection, err := models.CreateCollection(name, description, userName)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect("/collections/" + strconv.Itoa(collection.ID))
}

// HandleEditCollectionForm displays the edit collection form
func HandleEditCollectionForm(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil {
		return c.Status(404).SendString("Collection not found")
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return fiber.ErrForbidden
	}

	return HandleView(c, views.EditCollection(*collection))
}

// HandleUpdateCollection processes collection updates
func HandleUpdateCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil {
		return c.Status(404).SendString("Collection not found")
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return fiber.ErrForbidden
	}

	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))

	if name == "" {
		return c.Status(400).SendString("Collection name is required")
	}

	err = models.UpdateCollection(id, name, description)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect("/collections/" + strconv.Itoa(id))
}

// HandleDeleteCollection processes collection deletion
func HandleDeleteCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil {
		return c.Status(404).SendString("Collection not found")
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return fiber.ErrForbidden
	}

	err = models.DeleteCollection(id)
	if err != nil {
		return handleError(c, err)
	}

	return c.Redirect("/collections")
}

// HandleAddMediaToCollection adds media to a collection
func HandleAddMediaToCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil {
		return c.Status(404).SendString("Collection not found")
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return fiber.ErrForbidden
	}

	mediaSlug := c.FormValue("media_slug")
	if mediaSlug == "" {
		return c.Status(400).SendString("Media slug is required")
	}

	err = models.AddMediaToCollection(id, mediaSlug)
	if err != nil {
		return handleError(c, err)
	}

	return c.SendString("Media added to collection")
}

// HandleRemoveMediaFromCollection removes media from a collection
func HandleRemoveMediaFromCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil {
		return c.Status(404).SendString("Collection not found")
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return fiber.ErrForbidden
	}

	mediaSlug := c.Params("mediaSlug")
	if mediaSlug == "" {
		return c.Status(400).SendString("Media slug is required")
	}

	err = models.RemoveMediaFromCollection(id, mediaSlug)
	if err != nil {
		return handleError(c, err)
	}

	return c.SendString("Media removed from collection")
}

// HandleGetMediaCollections gets collections that contain a specific media
func HandleGetMediaCollections(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	if mediaSlug == "" {
		return c.Status(400).SendString("Media slug is required")
	}

	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Get user's collections
	collections, err := models.GetCollectionsByUser(userName)
	if err != nil {
		return handleError(c, err)
	}

	// Check which collections contain this media
	mediaCollections := []models.Collection{}
	for _, collection := range collections {
		isInCollection, err := models.IsMediaInCollection(collection.ID, mediaSlug)
		if err != nil {
			continue // Skip on error
		}
		if isInCollection {
			mediaCollections = append(mediaCollections, collection)
		}
	}

	return HandleView(c, views.MediaCollections(mediaSlug, collections, mediaCollections))
}

// HandleGetMediaCollectionsModal gets collections modal content for a specific media
func HandleGetMediaCollectionsModal(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	if mediaSlug == "" {
		return c.Status(400).SendString("Media slug is required")
	}

	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	// Get user's collections
	collections, err := models.GetCollectionsByUser(userName)
	if err != nil {
		return handleError(c, err)
	}

	// Check which collections contain this media
	mediaCollections := []models.Collection{}
	for _, collection := range collections {
		isInCollection, err := models.IsMediaInCollection(collection.ID, mediaSlug)
		if err != nil {
			continue // Skip on error
		}
		if isInCollection {
			mediaCollections = append(mediaCollections, collection)
		}
	}

	return HandleView(c, views.MediaCollectionsModal(mediaSlug, collections))
}

// HandleAddMediaToCollectionFromMedia adds media to a collection from the media page
func HandleAddMediaToCollectionFromMedia(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	if mediaSlug == "" {
		return c.Status(400).SendString("Media slug is required")
	}

	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	collectionIDStr := c.FormValue("collection_id")
	collectionID, err := strconv.Atoi(collectionIDStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	// Verify the collection belongs to the user
	collection, err := models.GetCollectionByID(collectionID)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil || collection.CreatedBy != userName {
		return fiber.ErrForbidden
	}

	// Check if media is already in collection
	alreadyInCollection, err := models.IsMediaInCollection(collectionID, mediaSlug)
	if err != nil {
		return handleError(c, err)
	}

	if !alreadyInCollection {
		err = models.AddMediaToCollection(collectionID, mediaSlug)
		if err != nil {
			return handleError(c, err)
		}
	}

	return HandleView(c, views.MediaCollectionItem(mediaSlug, *collection, true))
}

// HandleRemoveMediaFromCollectionFromMedia removes media from a collection from the media page
func HandleRemoveMediaFromCollectionFromMedia(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	if mediaSlug == "" {
		return c.Status(400).SendString("Media slug is required")
	}

	userName := GetUserContext(c)
	if userName == "" {
		return fiber.ErrUnauthorized
	}

	collectionIDStr := c.FormValue("collection_id")
	collectionID, err := strconv.Atoi(collectionIDStr)
	if err != nil {
		return c.Status(400).SendString("Invalid collection ID")
	}

	// Verify the collection belongs to the user
	collection, err := models.GetCollectionByID(collectionID)
	if err != nil {
		return handleError(c, err)
	}
	if collection == nil || collection.CreatedBy != userName {
		return fiber.ErrForbidden
	}

	// Check if media is in collection
	isInCollection, err := models.IsMediaInCollection(collectionID, mediaSlug)
	if err != nil {
		return handleError(c, err)
	}

	if isInCollection {
		err = models.RemoveMediaFromCollection(collectionID, mediaSlug)
		if err != nil {
			return handleError(c, err)
		}
	}

	return HandleView(c, views.MediaCollectionItem(mediaSlug, *collection, false))
}