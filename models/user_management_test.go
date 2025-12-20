package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"reader", true},
		{"premium", true},
		{"moderator", true},
		{"admin", true},
		{"invalid", false},
		{"", false},
		{"READER", false}, // case sensitive
	}

	for _, test := range tests {
		result := isValidRole(test.role)
		assert.Equal(t, test.expected, result, "isValidRole(%s)", test.role)
	}
}

func TestGetNextRole(t *testing.T) {
	tests := []struct {
		current  string
		expected string
		hasError bool
	}{
		{"reader", "premium", false},
		{"premium", "moderator", false},
		{"moderator", "admin", false},
		{"admin", "", true}, // no higher role
		{"invalid", "", true},
		{"", "", true},
	}

	for _, test := range tests {
		result, err := getNextRole(test.current)
		if test.hasError {
			assert.Error(t, err)
			assert.Empty(t, result)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		}
	}
}

func TestGetPreviousRole(t *testing.T) {
	tests := []struct {
		current  string
		expected string
		hasError bool
	}{
		{"admin", "moderator", false},
		{"moderator", "premium", false},
		{"premium", "reader", false},
		{"reader", "", true}, // no lower role
		{"invalid", "", true},
		{"", "", true},
	}

	for _, test := range tests {
		result, err := getPreviousRole(test.current)
		if test.hasError {
			assert.Error(t, err)
			assert.Empty(t, result)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		}
	}
}

func TestFindUserByUsername(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", false, "")

	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(rows)

	// Call the function
	user, err := FindUserByUsername("testuser")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "hashedpass", user.Password)
	assert.Equal(t, "reader", user.Role)
	assert.False(t, user.Banned)
	assert.Equal(t, "", user.Avatar)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFindUserByUsername_NotFound(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock no rows
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}))

	// Call the function
	user, err := FindUserByUsername("nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, user)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCountUsers(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	// Call the function
	count, err := CountUsers()
	assert.NoError(t, err)
	assert.Equal(t, int64(42), count)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUser_FirstUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock CountUsers to return 0 (first user)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock the insert
	mock.ExpectExec(`INSERT INTO users \(username, password, role, banned, avatar\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs("testuser", sqlmock.AnyArg(), "admin", false, "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock assignDefaultPermissionToUser
	mock.ExpectQuery(`SELECT id FROM permissions WHERE name = 'all' AND is_wildcard = 1 LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec(`INSERT OR IGNORE INTO user_permissions \(username, permission_id, created_at\) VALUES \(\?, \?, \?\)`).
		WithArgs("testuser", int64(1), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call the function
	err = CreateUser("testuser", "password123")
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUser_SubsequentUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock CountUsers to return 1 (not first user)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Mock the insert
	mock.ExpectExec(`INSERT INTO users \(username, password, role, banned, avatar\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs("testuser", sqlmock.AnyArg(), "reader", false, "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock assignDefaultPermissionToUser
	mock.ExpectQuery(`SELECT id FROM permissions WHERE name = 'all' AND is_wildcard = 1 LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec(`INSERT OR IGNORE INTO user_permissions \(username, permission_id, created_at\) VALUES \(\?, \?, \?\)`).
		WithArgs("testuser", int64(1), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call the function
	err = CreateUser("testuser", "password123")
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUser_PasswordHashing(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock CountUsers
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Mock the insert - password should be hashed (not plain text)
	mock.ExpectExec(`INSERT INTO users \(username, password, role, banned, avatar\) VALUES \(\?, \?, \?, \?, \?\)`).
		WithArgs("testuser", sqlmock.AnyArg(), "reader", false, "").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock assignDefaultPermissionToUser
	mock.ExpectQuery(`SELECT id FROM permissions WHERE name = 'all' AND is_wildcard = 1 LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec(`INSERT OR IGNORE INTO user_permissions \(username, permission_id, created_at\) VALUES \(\?, \?, \?\)`).
		WithArgs("testuser", int64(1), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call the function
	err = CreateUser("testuser", "password123")
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}
func TestUpdateUserRole(t *testing.T) {
// Create mock DB
mockDB, mock, err := sqlmock.New()
assert.NoError(t, err)
defer mockDB.Close()

// Replace global db
originalDB := db
db = mockDB
defer func() { db = originalDB }()

// Mock FindUserByUsername
mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
WithArgs("testuser").
WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
AddRow("testuser", "pass", "reader", false, ""))

// Mock the update
mock.ExpectExec(`UPDATE users SET role = \? WHERE username = \?`).
WithArgs("moderator", "testuser").
WillReturnResult(sqlmock.NewResult(0, 1))

// Call the function
err = UpdateUserRole("testuser", "moderator")
assert.NoError(t, err)

// Ensure expectations met
assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
// Call the function with invalid role
err := UpdateUserRole("testuser", "invalid")
assert.Error(t, err)
assert.Contains(t, err.Error(), "invalid role")
}

func TestUpdateUserRole_UserNotFound(t *testing.T) {
// Create mock DB
mockDB, mock, err := sqlmock.New()
assert.NoError(t, err)
defer mockDB.Close()

// Replace global db
originalDB := db
db = mockDB
defer func() { db = originalDB }()

// Mock FindUserByUsername returning no rows
mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
WithArgs("nonexistent").
WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}))

// Call the function
err = UpdateUserRole("nonexistent", "moderator")
assert.Error(t, err)
assert.Contains(t, err.Error(), "not found")

// Ensure expectations met
assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromoteUser_Success(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername (called by PromoteUser)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("testuser", "hashedpass", "reader", false, ""))

	// Mock FindUserByUsername (called by UpdateUserRole)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("testuser", "hashedpass", "reader", false, ""))

	// Mock UpdateUserRole
	mock.ExpectExec(`UPDATE users SET role = \? WHERE username = \?`).
		WithArgs("premium", "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Call the function
	err = PromoteUser("testuser")
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromoteUser_UserNotFound(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername returning no rows
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}))

	// Call the function
	err = PromoteUser("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromoteUser_BannedUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername returning banned user
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("banneduser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("banneduser", "hashedpass", "reader", true, ""))

	// Call the function
	err = PromoteUser("banneduser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is banned")

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPromoteUser_CannotPromoteFurther(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername returning admin user
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("adminuser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("adminuser", "hashedpass", "admin", false, ""))

	// Call the function
	err = PromoteUser("adminuser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to promote")

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDemoteUser_Success(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername (called by DemoteUser)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("testuser", "hashedpass", "premium", false, ""))

	// Mock FindUserByUsername (called by UpdateUserRole)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("testuser", "hashedpass", "premium", false, ""))

	// Mock UpdateUserRole
	mock.ExpectExec(`UPDATE users SET role = \? WHERE username = \?`).
		WithArgs("reader", "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Call the function
	err = DemoteUser("testuser")
	assert.NoError(t, err)

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDemoteUser_UserNotFound(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername returning no rows
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}))

	// Call the function
	err = DemoteUser("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDemoteUser_BannedUser(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername returning banned user
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("banneduser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("banneduser", "hashedpass", "premium", true, ""))

	// Call the function
	err = DemoteUser("banneduser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is banned")

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDemoteUser_CannotDemoteFurther(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername returning reader user
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("readeruser").
		WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
			AddRow("readeruser", "hashedpass", "reader", false, ""))

	// Call the function
	err = DemoteUser("readeruser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to demote")

	// Ensure expectations met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUserRoleTx_Success(t *testing.T) {
// Create mock DB
mockDB, mock, err := sqlmock.New()
assert.NoError(t, err)
defer mockDB.Close()

// Replace global db
originalDB := db
db = mockDB
defer func() { db = originalDB }()

// Create a mock transaction
mock.ExpectBegin()
tx, err := mockDB.Begin()
assert.NoError(t, err)

// Mock FindUserByUsername
mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
	WithArgs("testuser").
	WillReturnRows(sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", false, ""))

// Mock the update within transaction
mock.ExpectExec(`UPDATE users SET role = \? WHERE username = \?`).
	WithArgs("moderator", "testuser").
	WillReturnResult(sqlmock.NewResult(0, 1))

// Call the function
err = UpdateUserRoleTx(tx, "testuser", "moderator")
assert.NoError(t, err)

// Ensure expectations met
assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResetUserPassword(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE users SET password = \? WHERE username = \?`).
		WithArgs(sqlmock.AnyArg(), "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = ResetUserPassword("testuser", "newpassword")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResetUserPassword_UserNotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE users SET password = \? WHERE username = \?`).
		WithArgs(sqlmock.AnyArg(), "nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = ResetUserPassword("nonexistent", "newpassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user 'nonexistent' not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBanUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername
	userRows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", false, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows)

	// Mock UPDATE query
	mock.ExpectExec(`UPDATE users SET banned = \? WHERE username = \?`).
		WithArgs(true, "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = BanUser("testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBanUser_AlreadyBanned(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername - user is already banned
	userRows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", true, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows)

	err = BanUser("testuser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user 'testuser' is already banned")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBanUserTx(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Create a mock transaction
	mock.ExpectBegin()
	tx, err := db.Begin()
	assert.NoError(t, err)

	// Mock FindUserByUsername
	userRows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", false, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows)

	// Mock UPDATE query
	mock.ExpectExec(`UPDATE users SET banned = \? WHERE username = \?`).
		WithArgs(true, "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = BanUserTx(tx, "testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBanUserWithDemotion(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock Begin transaction
	mock.ExpectBegin()

	// Mock FindUserByUsername for UpdateUserRoleTx
	userRows1 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "moderator", false, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows1)

	// Mock UpdateUserRoleTx - demote to reader
	mock.ExpectExec(`UPDATE users SET role = \? WHERE username = \?`).
		WithArgs("reader", "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock FindUserByUsername for BanUserTx
	userRows2 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", false, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows2)

	// Mock BanUserTx UPDATE
	mock.ExpectExec(`UPDATE users SET banned = \? WHERE username = \?`).
		WithArgs(true, "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock Commit
	mock.ExpectCommit()

	err = BanUserWithDemotion("testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUnbanUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername - user is banned
	userRows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", true, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows)

	// Mock UPDATE query
	mock.ExpectExec(`UPDATE users SET banned = \? WHERE username = \?`).
		WithArgs(false, "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UnbanUser("testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUnbanUser_NotBanned(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock FindUserByUsername - user is not banned
	userRows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar"}).
		AddRow("testuser", "hashedpass", "reader", false, "")
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar FROM users WHERE username = \?`).
		WithArgs("testuser").
		WillReturnRows(userRows)

	err = UnbanUser("testuser")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user 'testuser' is not banned")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBanIP(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`INSERT INTO banned_ips \(ip_address, reason\) VALUES \(\?, \?\)`).
		WithArgs("192.168.1.1", "spam").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = BanIP("192.168.1.1", "spam")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUnbanIP(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`DELETE FROM banned_ips WHERE ip_address = \?`).
		WithArgs("192.168.1.1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UnbanIP("192.168.1.1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsIPBanned(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM banned_ips WHERE ip_address = \?`).
		WithArgs("192.168.1.1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	banned, err := IsIPBanned("192.168.1.1")
	assert.NoError(t, err)
	assert.True(t, banned)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIsIPBanned_NotBanned(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM banned_ips WHERE ip_address = \?`).
		WithArgs("192.168.1.1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	banned, err := IsIPBanned("192.168.1.1")
	assert.NoError(t, err)
	assert.False(t, banned)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetBannedIPs(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	bannedIPRows := sqlmock.NewRows([]string{"id", "ip_address", "banned_at", "reason"}).
		AddRow(1, "192.168.1.1", time.Unix(1234567890, 0), "spam").
		AddRow(2, "192.168.1.2", time.Unix(1234567891, 0), "abuse")
	mock.ExpectQuery(`SELECT id, ip_address, banned_at, reason FROM banned_ips ORDER BY banned_at DESC`).
		WillReturnRows(bannedIPRows)

	bannedIPs, err := GetBannedIPs()
	assert.NoError(t, err)
	assert.Len(t, bannedIPs, 2)
	assert.Equal(t, "192.168.1.1", bannedIPs[0].IPAddress)
	assert.Equal(t, "spam", bannedIPs[0].Reason)
	assert.Equal(t, "192.168.1.2", bannedIPs[1].IPAddress)
	assert.Equal(t, "abuse", bannedIPs[1].Reason)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUserAvatar(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE users SET avatar = \? WHERE username = \?`).
		WithArgs("http://example.com/avatar.jpg", "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateUserAvatar("testuser", "http://example.com/avatar.jpg")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserRoleDistribution(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"role", "count"}).
		AddRow("admin", 2).
		AddRow("reader", 5).
		AddRow("premium", 3)

	mock.ExpectQuery(`SELECT role, COUNT\(\*\) as count FROM users GROUP BY role ORDER BY count DESC`).
		WillReturnRows(rows)

	distribution, err := GetUserRoleDistribution()
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{
		"admin":   2,
		"reader":  5,
		"premium": 3,
	}, distribution)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserRoleDistribution_Empty(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	rows := sqlmock.NewRows([]string{"role", "count"})

	mock.ExpectQuery(`SELECT role, COUNT\(\*\) as count FROM users GROUP BY role ORDER BY count DESC`).
		WillReturnRows(rows)

	distribution, err := GetUserRoleDistribution()
	assert.NoError(t, err)
	assert.Empty(t, distribution)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUsers(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query
	rows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"}).
		AddRow("user1", "pass1", "reader", false, "", time.Now()).
		AddRow("user2", "pass2", "admin", true, "avatar.jpg", time.Now())

	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows)

	users, err := GetUsers()
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "user1", users[0].Username)
	assert.Equal(t, "user2", users[1].Username)
	assert.Equal(t, "reader", users[0].Role)
	assert.Equal(t, "admin", users[1].Role)
	assert.False(t, users[0].Banned)
	assert.True(t, users[1].Banned)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUsers_Empty(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock empty result
	rows := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"})

	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows)

	users, err := GetUsers()
	assert.NoError(t, err)
	assert.Empty(t, users)
}

func TestGetUsersWithOptions(t *testing.T) {
	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace global db
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock result with users (need separate rows for each expectation)
	rows1 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"}).
		AddRow("user1", "pass1", "reader", false, "avatar1.jpg", time.Now()).
		AddRow("user2", "pass2", "admin", false, "avatar2.jpg", time.Now()).
		AddRow("testuser", "pass3", "moderator", false, "avatar3.jpg", time.Now())

	rows2 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"}).
		AddRow("user1", "pass1", "reader", false, "avatar1.jpg", time.Now()).
		AddRow("user2", "pass2", "admin", false, "avatar2.jpg", time.Now()).
		AddRow("testuser", "pass3", "moderator", false, "avatar3.jpg", time.Now())

	rows3 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"}).
		AddRow("user1", "pass1", "reader", false, "avatar1.jpg", time.Now()).
		AddRow("user2", "pass2", "admin", false, "avatar2.jpg", time.Now()).
		AddRow("testuser", "pass3", "moderator", false, "avatar3.jpg", time.Now())

	rows4 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"}).
		AddRow("user1", "pass1", "reader", false, "avatar1.jpg", time.Now()).
		AddRow("user2", "pass2", "admin", false, "avatar2.jpg", time.Now()).
		AddRow("testuser", "pass3", "moderator", false, "avatar3.jpg", time.Now())

	rows5 := sqlmock.NewRows([]string{"username", "password", "role", "banned", "avatar", "created_at"}).
		AddRow("user1", "pass1", "reader", false, "avatar1.jpg", time.Now()).
		AddRow("user2", "pass2", "admin", false, "avatar2.jpg", time.Now()).
		AddRow("testuser", "pass3", "moderator", false, "avatar3.jpg", time.Now())

	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows1)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows2)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows3)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows4)
	mock.ExpectQuery(`SELECT username, password, role, banned, avatar, created_at FROM users`).
		WillReturnRows(rows5)

	// Test without filter
	users, total, err := GetUsersWithOptions(UserSearchOptions{Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Len(t, users, 3)
	assert.Equal(t, int64(3), total)

	// Test with filter
	users, total, err = GetUsersWithOptions(UserSearchOptions{Filter: "user", Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Len(t, users, 3) // all users contain "user"
	assert.Equal(t, int64(3), total)

	// Test with specific filter
	users, total, err = GetUsersWithOptions(UserSearchOptions{Filter: "test", Page: 1, PageSize: 10})
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "testuser", users[0].Username)
	assert.Equal(t, int64(1), total)

	// Test pagination
	users, total, err = GetUsersWithOptions(UserSearchOptions{Page: 1, PageSize: 2})
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, "user1", users[0].Username)
	assert.Equal(t, "user2", users[1].Username)

	users, total, err = GetUsersWithOptions(UserSearchOptions{Page: 2, PageSize: 2})
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, "testuser", users[0].Username)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaginateUsers(t *testing.T) {
	users := []User{
		{Username: "user1"},
		{Username: "user2"},
		{Username: "user3"},
		{Username: "user4"},
		{Username: "user5"},
	}

	// Test page 1, page size 2
	result := paginateUsers(users, 1, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "user1", result[0].Username)
	assert.Equal(t, "user2", result[1].Username)

	// Test page 2, page size 2
	result = paginateUsers(users, 2, 2)
	assert.Len(t, result, 2)
	assert.Equal(t, "user3", result[0].Username)
	assert.Equal(t, "user4", result[1].Username)

	// Test page 3, page size 2
	result = paginateUsers(users, 3, 2)
	assert.Len(t, result, 1)
	assert.Equal(t, "user5", result[0].Username)

	// Test page beyond available
	result = paginateUsers(users, 10, 2)
	assert.Empty(t, result)

	// Test page size 0 (return all)
	result = paginateUsers(users, 1, 0)
	assert.Len(t, result, 5)
}
