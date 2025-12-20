package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreatePermission(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`INSERT INTO permissions`).
		WithArgs("test_perm", "Test permission", true, true, true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	perm, err := CreatePermission("test_perm", "Test permission", true, true)
	assert.NoError(t, err)
	assert.NotNil(t, perm)
	assert.Equal(t, int64(1), perm.ID)
	assert.Equal(t, "test_perm", perm.Name)
	assert.Equal(t, "Test permission", perm.Description)
	assert.True(t, perm.IsWildcard)
	assert.True(t, perm.PremiumChapterAccess)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPermissions(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"id", "name", "description", "is_wildcard", "is_enabled", "premium_chapter_access", "created_at", "updated_at"}).
		AddRow(1, "perm1", "Permission 1", true, true, false, 1234567890, 1234567890).
		AddRow(2, "perm2", "Permission 2", false, true, true, 1234567890, 1234567890)

	mock.ExpectQuery(`SELECT id, name, description, is_wildcard, is_enabled, premium_chapter_access, created_at, updated_at FROM permissions ORDER BY name`).
		WillReturnRows(rows)

	perms, err := GetPermissions()
	assert.NoError(t, err)
	assert.Len(t, perms, 2)
	assert.Equal(t, "perm1", perms[0].Name)
	assert.Equal(t, "perm2", perms[1].Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPermission(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	row := sqlmock.NewRows([]string{"id", "name", "description", "is_wildcard", "is_enabled", "premium_chapter_access", "created_at", "updated_at"}).
		AddRow(1, "test_perm", "Test permission", true, true, false, 1234567890, 1234567890)

	mock.ExpectQuery(`SELECT id, name, description, is_wildcard, is_enabled, premium_chapter_access, created_at, updated_at FROM permissions WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnRows(row)

	perm, err := GetPermission(1)
	assert.NoError(t, err)
	assert.NotNil(t, perm)
	assert.Equal(t, int64(1), perm.ID)
	assert.Equal(t, "test_perm", perm.Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePermission(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE permissions SET name = \?, description = \?, is_wildcard = \?, is_enabled = \?, premium_chapter_access = \?, updated_at = \? WHERE id = \?`).
		WithArgs("updated_perm", "Updated permission", false, true, true, sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdatePermission(1, "updated_perm", "Updated permission", false, true, true)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeletePermission(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`DELETE FROM permissions WHERE id = \?`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeletePermission(1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}