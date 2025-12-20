package handlers

import (
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/fiber/v2"
)

// CollectionMediaFormData represents form data for adding/removing media from collections
type CollectionMediaFormData struct {
	MediaSlug    string `json:"media_slug"`
	CollectionID string `json:"collection_id"`
}

// HandleCollections displays all collections
func HandleCollections(c *fiber.Ctx) error {
	collections, err := models.GetAllCollections()
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return HandleView(c, views.Collections(collections))
}

// HandleUserCollections displays collections created by the current user
func HandleUserCollections(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	collections, err := models.GetCollectionsByUser(userName)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return HandleView(c, views.Collections(collections))
}

// HandleCollection displays a specific collection
func HandleCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	media, err := models.GetCollectionMedia(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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

// HandleCreateCollectionModal displays the create collection form as a modal
func HandleCreateCollectionModal(c *fiber.Ctx) error {
	return HandleView(c, views.CreateCollectionModal())
}

// HandleCreateCollection processes collection creation
func HandleCreateCollection(c *fiber.Ctx) error {
	userName := GetUserContext(c)
	if userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	var formData models.Collection
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	name := strings.TrimSpace(formData.Name)
	description := strings.TrimSpace(formData.Description)

	if name == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	collection, err := models.CreateCollection(name, description, userName)
	if err != nil {
		return sendInternalServerError(c, ErrCollectionCreateFailed, err)
	}

	// Return success response for HTMX (modal submissions)
	if c.Get("HX-Request") == "true" {
		triggerCustomNotification(c, "collectionCreated", map[string]interface{}{
			"message": "Collection created successfully",
			"status":  "success",
		})
		return c.SendString("")
	}

	return c.Redirect("/collections/" + strconv.Itoa(collection.ID))
}

// HandleEditCollectionForm displays the edit collection form
func HandleEditCollectionForm(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return sendForbiddenError(c, ErrForbidden)
	}

	return HandleView(c, views.EditCollection(*collection))
}

// HandleEditCollectionModal displays the edit collection form as a modal
func HandleEditCollectionModal(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return sendForbiddenError(c, ErrForbidden)
	}

	return HandleView(c, views.EditCollectionModal(*collection))
}

// HandleUpdateCollection processes collection updates
func HandleUpdateCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return sendForbiddenError(c, ErrForbidden)
	}

	var formData models.Collection
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	name := strings.TrimSpace(formData.Name)
	description := strings.TrimSpace(formData.Description)

	if name == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	err = models.UpdateCollection(id, name, description)
	if err != nil {
		return sendInternalServerError(c, ErrCollectionUpdateFailed, err)
	}

	// Return success response for HTMX (modal submissions)
	if c.Get("HX-Request") == "true" {
		triggerCustomNotification(c, "collectionUpdated", map[string]interface{}{
			"message": "Collection updated successfully",
			"status":  "success",
		})
		return c.SendString("")
	}

	return c.Redirect("/collections/" + strconv.Itoa(id))
}

// HandleDeleteCollection processes collection deletion
func HandleDeleteCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return sendForbiddenError(c, ErrForbidden)
	}

	err = models.DeleteCollection(id)
	if err != nil {
		return sendInternalServerError(c, ErrCollectionDeleteFailed, err)
	}

	return c.Redirect("/collections")
}

// HandleAddMediaToCollection adds media to a collection
func HandleAddMediaToCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return sendForbiddenError(c, ErrForbidden)
	}

	var formData CollectionMediaFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	mediaSlug := formData.MediaSlug
	if mediaSlug == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	err = models.AddMediaToCollection(id, mediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return c.SendString("Media added to collection")
}

// HandleRemoveMediaFromCollection removes media from a collection
func HandleRemoveMediaFromCollection(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	collection, err := models.GetCollectionByID(id)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil {
		return sendNotFoundError(c, ErrCollectionNotFound)
	}

	userName := GetUserContext(c)
	if userName == "" || (userName != collection.CreatedBy && userName != "admin" && userName != "moderator") {
		return sendForbiddenError(c, ErrForbidden)
	}

	mediaSlug := c.Params("mediaSlug")
	if mediaSlug == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	err = models.RemoveMediaFromCollection(id, mediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	return c.SendString("")
}

// HandleGetMediaCollections gets collections that contain a specific media
func HandleGetMediaCollections(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	if mediaSlug == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	userName := GetUserContext(c)
	if userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	// Get user's collections
	collections, err := models.GetCollectionsByUser(userName)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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
		return sendValidationError(c, ErrRequiredField)
	}

	userName := GetUserContext(c)
	if userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	// Get user's collections
	collections, err := models.GetCollectionsByUser(userName)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
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
		return sendValidationError(c, ErrRequiredField)
	}

	userName := GetUserContext(c)
	if userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	var formData CollectionMediaFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	collectionID, err := strconv.Atoi(formData.CollectionID)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	// Verify the collection belongs to the user
	collection, err := models.GetCollectionByID(collectionID)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil || collection.CreatedBy != userName {
		return sendForbiddenError(c, ErrForbidden)
	}

	// Check if media is already in collection
	alreadyInCollection, err := models.IsMediaInCollection(collectionID, mediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	if !alreadyInCollection {
		err = models.AddMediaToCollection(collectionID, mediaSlug)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
	}

	return HandleView(c, views.MediaCollectionItem(mediaSlug, *collection, true))
}

// HandleRemoveMediaFromCollectionFromMedia removes media from a collection from the media page
func HandleRemoveMediaFromCollectionFromMedia(c *fiber.Ctx) error {
	mediaSlug := c.Params("media")
	if mediaSlug == "" {
		return sendValidationError(c, ErrRequiredField)
	}

	userName := GetUserContext(c)
	if userName == "" {
		return sendUnauthorizedError(c, ErrUnauthorized)
	}

	var formData CollectionMediaFormData
	if err := c.BodyParser(&formData); err != nil {
		return sendBadRequestError(c, ErrBadRequest)
	}

	collectionID, err := strconv.Atoi(formData.CollectionID)
	if err != nil {
		return sendBadRequestError(c, ErrInvalidCollectionID)
	}

	// Verify the collection belongs to the user
	collection, err := models.GetCollectionByID(collectionID)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}
	if collection == nil || collection.CreatedBy != userName {
		return sendForbiddenError(c, ErrForbidden)
	}

	// Check if media is in collection
	isInCollection, err := models.IsMediaInCollection(collectionID, mediaSlug)
	if err != nil {
		return sendInternalServerError(c, ErrInternalServerError, err)
	}

	if isInCollection {
		err = models.RemoveMediaFromCollection(collectionID, mediaSlug)
		if err != nil {
			return sendInternalServerError(c, ErrInternalServerError, err)
		}
	}

	return HandleView(c, views.MediaCollectionItem(mediaSlug, *collection, false))
}