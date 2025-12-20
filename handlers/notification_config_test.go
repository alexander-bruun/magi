package handlers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNotificationStatusForHTTPStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   NotificationStatus
	}{
		{200, StatusSuccess},     // OK
		{201, StatusSuccess},     // Created
		{299, StatusSuccess},     // Last 2xx
		{300, StatusInfo},        // Redirect (not in 2xx, 4xx, 5xx)
		{400, StatusWarning},     // Bad Request
		{401, StatusDestructive}, // Unauthorized
		{403, StatusDestructive}, // Forbidden
		{404, StatusWarning},     // Not Found
		{499, StatusWarning},     // Last 4xx
		{500, StatusDestructive}, // Internal Server Error
		{502, StatusDestructive}, // Bad Gateway
		{599, StatusDestructive}, // Last 5xx
		{100, StatusInfo},        // Continue (not in ranges)
		{0, StatusInfo},          // Invalid status
	}

	for _, test := range tests {
		result := GetNotificationStatusForHTTPStatus(test.statusCode)
		assert.Equal(t, test.expected, result, "GetNotificationStatusForHTTPStatus(%d)", test.statusCode)
	}
}

func TestGetNotificationStatusForError(t *testing.T) {
	// Test nil error
	result := GetNotificationStatusForError(nil)
	assert.Equal(t, StatusSuccess, result)

	// Test any error
	err := errors.New("test error")
	result = GetNotificationStatusForError(err)
	assert.Equal(t, StatusDestructive, result)
}