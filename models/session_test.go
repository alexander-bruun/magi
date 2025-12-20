package models

import (
	"database/sql"
	"encoding/base64"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomKey(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "generate key of length 16",
			length: 16,
		},
		{
			name:   "generate key of length 32",
			length: 32,
		},
		{
			name:   "generate key of length 64",
			length: 64,
		},
		{
			name:   "generate key of length 0",
			length: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateRandomKey(tt.length)
			assert.NoError(t, err)

			// Verify the key can be decoded and has the expected length
			decoded, err := base64.StdEncoding.DecodeString(key)
			assert.NoError(t, err)
			assert.Equal(t, tt.length, len(decoded))

			// For non-zero length, key should not be empty
			if tt.length > 0 {
				assert.NotEmpty(t, key)
			}
		})
	}

	// Test that different calls produce different keys
	key1, err1 := GenerateRandomKey(16)
	key2, err2 := GenerateRandomKey(16)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NotEqual(t, key1, key2)
}

func TestCreateSessionToken(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`INSERT INTO session_tokens`).
		WithArgs(sqlmock.AnyArg(), "testuser", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	token, err := CreateSessionToken("testuser")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSessionToken(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the SELECT query
	row := sqlmock.NewRows([]string{"username", "expires_at"}).
		AddRow("testuser", time.Now().Add(time.Hour)) // Not expired

	mock.ExpectQuery(`SELECT username, expires_at FROM session_tokens WHERE token = \?`).
		WithArgs("validtoken").
		WillReturnRows(row)

	// Mock the UPDATE query for last_used_at
	mock.ExpectExec(`UPDATE session_tokens SET last_used_at = \? WHERE token = \?`).
		WithArgs(sqlmock.AnyArg(), "validtoken").
		WillReturnResult(sqlmock.NewResult(0, 1))

	username, err := ValidateSessionToken("validtoken")
	assert.NoError(t, err)
	assert.Equal(t, "testuser", username)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateSessionTokenInvalid(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT username, expires_at FROM session_tokens WHERE token = \?`).
		WithArgs("invalidtoken").
		WillReturnError(sql.ErrNoRows)

	username, err := ValidateSessionToken("invalidtoken")
	assert.Error(t, err)
	assert.Equal(t, "", username)
	assert.Contains(t, err.Error(), "invalid session token")

	assert.NoError(t, mock.ExpectationsWereMet())
}