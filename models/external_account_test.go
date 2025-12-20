package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetUserExternalAccount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	row := sqlmock.NewRows([]string{"id", "user_name", "service_name", "external_user_id", "access_token", "refresh_token", "token_expires_at", "created_at", "updated_at"}).
		AddRow(1, "testuser", "anilist", "12345", "token123", "refresh123", time.Unix(1234567890, 0), time.Unix(1234567890, 0), time.Unix(1234567890, 0))

	mock.ExpectQuery(`SELECT id, user_name, service_name, external_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at FROM user_external_accounts WHERE user_name = \? AND service_name = \?`).
		WithArgs("testuser", "anilist").
		WillReturnRows(row)

	account, err := GetUserExternalAccount("testuser", "anilist")
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, "testuser", account.UserName)
	assert.Equal(t, "anilist", account.ServiceName)
	assert.Equal(t, "12345", account.ExternalUserID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveUserExternalAccount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	account := &UserExternalAccount{
		UserName:       "testuser",
		ServiceName:    "anilist",
		ExternalUserID: "12345",
		AccessToken:    "token123",
		RefreshToken:   "refresh123",
		TokenExpiresAt: time.Unix(1234567890, 0),
		CreatedAt:      time.Unix(1234567890, 0),
		UpdatedAt:      time.Unix(1234567890, 0),
	}

	mock.ExpectExec(`INSERT INTO user_external_accounts`).
		WithArgs(account.UserName, account.ServiceName, account.ExternalUserID, account.AccessToken, account.RefreshToken, account.TokenExpiresAt, account.CreatedAt, account.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = SaveUserExternalAccount(account)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteUserExternalAccount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`DELETE FROM user_external_accounts WHERE user_name = \? AND service_name = \?`).
		WithArgs("testuser", "anilist").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteUserExternalAccount("testuser", "anilist")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserExternalAccounts(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"id", "user_name", "service_name", "external_user_id", "access_token", "refresh_token", "token_expires_at", "created_at", "updated_at"}).
		AddRow(1, "testuser", "anilist", "12345", "token123", "refresh123", time.Unix(1234567890, 0), time.Unix(1234567890, 0), time.Unix(1234567890, 0)).
		AddRow(2, "testuser", "mal", "67890", "token456", "refresh456", time.Unix(1234567891, 0), time.Unix(1234567890, 0), time.Unix(1234567890, 0))

	mock.ExpectQuery(`SELECT id, user_name, service_name, external_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at FROM user_external_accounts WHERE user_name = \?`).
		WithArgs("testuser").
		WillReturnRows(rows)

	accounts, err := GetUserExternalAccounts("testuser")
	assert.NoError(t, err)
	assert.Len(t, accounts, 2)
	assert.Equal(t, "anilist", accounts[0].ServiceName)
	assert.Equal(t, "mal", accounts[1].ServiceName)

	assert.NoError(t, mock.ExpectationsWereMet())
}