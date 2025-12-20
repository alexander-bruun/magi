package handlers

// Common user-facing error messages
const (
	// Generic errors
	ErrInternalServerError = "An unexpected error occurred. Please try again later."
	ErrBadRequest          = "Invalid request. Please check your input and try again."

	// Authentication errors
	ErrUnauthorized        = "You must be logged in to perform this action."
	ErrInvalidCredentials  = "Invalid username or password."
	ErrSessionExpired      = "Your session has expired. Please log in again."

	// Authorization errors
	ErrForbidden           = "You don't have permission to perform this action."
	ErrAccessDenied        = "Access denied."
	ErrSuspiciousActivity  = "Access denied: suspicious activity detected."

	// Validation errors
	ErrValidationFailed    = "Please correct the errors below and try again."
	ErrRequiredField       = "This field is required."
	ErrInvalidFormat       = "Invalid format."
	ErrDuplicateEntry      = "This item already exists."

	// Library errors
	ErrLibraryNotFound     = "Library not found."
	ErrLibraryCreateFailed = "Failed to create library."
	ErrLibraryUpdateFailed = "Failed to update library."
	ErrLibraryDeleteFailed = "Failed to delete library."
	ErrInvalidCron         = "Invalid cron expression (must be 5 fields: minute hour day month weekday)."
	ErrFolderNotExist      = "Folder does not exist."
	ErrFolderNotAccessible = "Cannot access folder."
	ErrPathNotDirectory    = "Path is not a directory."

	// Media errors
	ErrMediaNotFound       = "Media not found."
	ErrMediaDeleteFailed   = "Failed to delete media."

	// Comment errors
	ErrCommentCreateFailed = "Failed to create comment."
	ErrCommentUpdateFailed = "Failed to update comment."
	ErrCommentDeleteFailed = "Failed to delete comment."
	ErrCommentNotFound     = "Comment not found."
	ErrEmptyComment        = "Comment content cannot be empty."

	// User errors
	ErrUserNotFound        = "User not found."
	ErrLoginFailed         = "Invalid username or password."
	ErrUsernameExists      = "Username already exists."
	ErrUsernameTooShort    = "Username must be at least 3 characters long."
	ErrUsernameTooLong     = "Username cannot be longer than 50 characters."
	ErrPasswordTooWeak     = "Password must be at least 8 characters long and contain uppercase, lowercase, and numbers."
	ErrMaxUsersReached     = "Maximum number of users reached."

	// Chapter errors
	ErrChapterNotFound         = "Chapter not found."
	ErrChapterAccessDenied     = "Access denied: you don't have permission to view this chapter."
	ErrChapterPremiumAccess    = "This chapter is in premium early access. Please wait for it to be released or upgrade your account."
	ErrChapterRemoved          = "This chapter is no longer available and has been removed."
	ErrChapterFileReadFailed   = "Failed to read chapter file."
	ErrChapterImageProcessFailed = "Failed to process chapter images."
	ErrChapterRenderFailed     = "Failed to render chapter."

	// Scraper errors
	ErrInvalidScriptID        = "Invalid script ID."
	ErrInvalidLogID           = "Invalid log ID."
	ErrScraperScriptNotFound  = "Scraper script not found."
	ErrScraperScriptInvalid   = "Invalid scraper script."
	ErrScraperExecutionFailed = "Failed to execute scraper script."

	// Review errors
	ErrReviewNotFound     = "Review not found."
	ErrReviewCreateFailed = "Failed to create review."
	ErrReviewUpdateFailed = "Failed to update review."
	ErrReviewDeleteFailed = "Failed to delete review."
	ErrInvalidReviewID    = "Invalid review ID."
	ErrInvalidRating      = "Rating must be between 1 and 10."

	// Metadata errors
	ErrMetadataSyncFailed     = "Failed to sync metadata."
	ErrMetadataProviderError  = "Error from metadata provider."
	ErrMetadataUpdateFailed   = "Failed to update metadata."
	ErrMetadataSearchFailed   = "Failed to search metadata."
	ErrInvalidMetadataID      = "Invalid metadata ID."

	// Collection errors
	ErrCollectionNotFound     = "Collection not found."
	ErrCollectionCreateFailed = "Failed to create collection."
	ErrCollectionUpdateFailed = "Failed to update collection."
	ErrCollectionDeleteFailed = "Failed to delete collection."
	ErrInvalidCollectionID    = "Invalid collection ID."

	// Comic errors
	ErrComicTokenRequired     = "Access token is required to view this comic."
	ErrComicTokenInvalid      = "Invalid or expired access token."
	ErrComicNotFound          = "Comic not found."

	// Config errors
	ErrConfigLoadFailed       = "Failed to load configuration."
	ErrConfigUpdateFailed     = "Failed to update configuration."

	// Notification errors
	ErrNotificationOperationFailed = "Failed to perform notification operation."

	// Chapter errors
	ErrInvalidMediaSlug       = "Invalid media identifier."
	ErrChapterReleaseFailed   = "Failed to release chapter."

	// Backup errors
	ErrBackupListFailed       = "Failed to retrieve backup list."
	ErrBackupCreateFailed     = "Failed to create backup."
	ErrBackupRestoreFailed    = "Failed to restore backup."

	// Media interaction errors
	ErrMediaVoteFailed        = "Failed to process vote."
	ErrMediaFavoriteFailed    = "Failed to update favorite status."

	// Image errors
	ErrImageTokenRequired     = "Access token is required to view this image."
	ErrImageTokenInvalid      = "Invalid or expired access token."
	ErrImageNotFound          = "Image not found."
	ErrImageAccessDenied      = "Access denied: you don't have permission to view this image."
	ErrImageProcessingFailed  = "Failed to process image."
	ErrImageUnsupportedType   = "Unsupported image file type."

	// Poster errors
	ErrPosterProcessingFailed = "Failed to process poster."
	ErrPosterUploadFailed     = "Failed to upload poster."
	ErrPosterSaveFailed       = "Failed to save poster."

	// Permission errors
	ErrPermissionNotFound     = "Permission not found."
	ErrPermissionInvalidID    = "Invalid permission ID."
	ErrPermissionNameRequired = "Permission name is required."
	ErrPermissionOperationFailed = "Failed to perform permission operation."
	ErrPermissionUserRequired = "Username and permission are required."
	ErrPermissionRoleRequired = "Role and permission are required."

	// User management errors
	ErrUserManagementOperationFailed = "Failed to perform user management operation."
	ErrOAuthCredentialsRequired      = "OAuth credentials are required."
	ErrOAuthClientNotConfigured      = "OAuth client not configured."
	ErrOAuthNoCredentials            = "No OAuth credentials found."
	ErrOAuthMissingCodeOrState       = "Missing authorization code or state."
	ErrOAuthInvalidState             = "Invalid authorization state."
	ErrOAuthNoAccountFound           = "No account found for OAuth provider."
	ErrOAuthInvalidStoredState       = "Invalid stored authorization state."
	ErrOAuthStateMismatch            = "Authorization state mismatch."
	ErrOAuthTokenExchangeFailed      = "Failed to exchange authorization token."

	// Rate limiting
	ErrRateLimitExceeded      = "Too many requests. Please wait before trying again."

	// Bot detection
	ErrBotDetected            = "Unusual activity detected. Please try again in a moment."
)