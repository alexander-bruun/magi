package handlers

// NotificationStatus represents the status levels for notifications
type NotificationStatus string

const (
	// StatusSuccess indicates a successful operation
	StatusSuccess NotificationStatus = "success"
	// StatusWarning indicates a warning or non-critical error
	StatusWarning NotificationStatus = "warning"
	// StatusDestructive indicates a critical error or failure
	StatusDestructive NotificationStatus = "destructive"
	// StatusInfo indicates informational messages
	StatusInfo NotificationStatus = "info"
)

// GetNotificationStatusForHTTPStatus maps HTTP status codes to notification statuses
func GetNotificationStatusForHTTPStatus(statusCode int) NotificationStatus {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return StatusSuccess
	case statusCode >= 400 && statusCode < 500:
		if statusCode == 401 || statusCode == 403 {
			return StatusDestructive
		}
		return StatusWarning
	case statusCode >= 500:
		return StatusDestructive
	default:
		return StatusInfo
	}
}

// GetNotificationStatusForError maps common error types to notification statuses
func GetNotificationStatusForError(err error) NotificationStatus {
	if err == nil {
		return StatusSuccess
	}

	// This could be expanded to check for specific error types
	// For now, default to destructive for any error
	return StatusDestructive
}
