package models

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateUserNotification(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	// Replace the global db with mock
	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`INSERT INTO user_notifications`).
		WithArgs("testuser", "manga1", "chapter1", "New chapter available", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = CreateUserNotification("testuser", "manga1", "chapter1", "New chapter available")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserNotifications(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"id", "user_name", "media_slug", "chapter_slug", "message", "is_read", "created_at", "manga_name", "chapter_name"}).
		AddRow(1, "testuser", "manga1", "chapter1", "New chapter", false, 1640995200, "Manga One", "Chapter 1").
		AddRow(2, "testuser", "manga2", "chapter2", "Another chapter", true, 1641081600, "Manga Two", "Chapter 2")

	mock.ExpectQuery(`SELECT n\.id, n\.user_name, n\.media_slug, n\.chapter_slug, n\.message, n\.is_read, n\.created_at, m\.name as manga_name, c\.name as chapter_name FROM user_notifications n LEFT JOIN media m ON n\.media_slug = m\.slug LEFT JOIN chapters c ON n\.chapter_slug = c\.slug AND n\.media_slug = c\.media_slug WHERE n\.user_name = \? ORDER BY n\.created_at DESC LIMIT 50`).
		WithArgs("testuser").
		WillReturnRows(rows)

	notifications, err := GetUserNotifications("testuser", false)
	assert.NoError(t, err)
	assert.Len(t, notifications, 2)
	assert.Equal(t, "manga1", notifications[0].MediaSlug)
	assert.Equal(t, "manga2", notifications[1].MediaSlug)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUnreadNotificationCount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_notifications WHERE user_name = \? AND is_read = 0`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	count, err := GetUnreadNotificationCount("testuser")
	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkNotificationAsRead(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE user_notifications SET is_read = 1 WHERE id = \? AND user_name = \?`).
		WithArgs(int64(1), "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = MarkNotificationAsRead(1, "testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkAllNotificationsAsRead(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`UPDATE user_notifications SET is_read = 1 WHERE user_name = \? AND is_read = 0`).
		WithArgs("testuser").
		WillReturnResult(sqlmock.NewResult(0, 3))

	err = MarkAllNotificationsAsRead("testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteNotification(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`DELETE FROM user_notifications WHERE id = \? AND user_name = \?`).
		WithArgs(int64(1), "testuser").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteNotification(1, "testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClearReadNotifications(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectExec(`DELETE FROM user_notifications WHERE user_name = \? AND is_read = 1`).
		WithArgs("testuser").
		WillReturnResult(sqlmock.NewResult(0, 5))

	err = ClearReadNotifications("testuser")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotifyUsersOfNewChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock BeginTx
	mock.ExpectBegin()

	// Mock chapter query
	chapterRows := sqlmock.NewRows([]string{"slug", "name"}).
		AddRow("ch1", "Chapter 1").
		AddRow("ch2", "Chapter 2")
	mock.ExpectQuery(`SELECT c.slug, c.name FROM chapters c WHERE c.media_slug = \? AND c.slug IN .*`).
		WithArgs("manga1", "ch1", "ch2").
		WillReturnRows(chapterRows)

	// Mock users query
	userRows := sqlmock.NewRows([]string{"user_name"}).
		AddRow("user1").
		AddRow("user2")
	mock.ExpectQuery(`SELECT DISTINCT user_name FROM reading_states WHERE media_slug = \?`).
		WithArgs("manga1").
		WillReturnRows(userRows)

	// Mock GetMediaUnfiltered query
	mediaRows := sqlmock.NewRows([]string{"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "created_at", "updated_at"}).
		AddRow("manga1", "Manga One", "Author", "Description", 2023, "en", "manga", "ongoing", "safe", "lib1", "cover.jpg", "/path", 10, 1234567890, 1234567890)
	mock.ExpectQuery(`SELECT m\.slug, m\.name, m\.author, m\.description, m\.year, m\.original_language, m\.type, m\.status, m\.content_rating, m\.library_slug, m\.cover_art_url, m\.path, m\.file_count, m\.created_at, m\.updated_at FROM media m JOIN libraries l ON m\.library_slug = l\.slug WHERE m\.slug = \? AND l\.enabled = 1`).
		WithArgs("manga1").
		WillReturnRows(mediaRows)

	// Mock GetTagsForMedia query
	tagRows := sqlmock.NewRows([]string{"tag"})
	mock.ExpectQuery(`SELECT tag FROM media_tags WHERE media_slug = \? ORDER BY tag`).
		WithArgs("manga1").
		WillReturnRows(tagRows)

	// Mock the exists query (no recent notification) for user1
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_notifications WHERE user_name = \? AND media_slug = \? AND chapter_slug = \? AND created_at > \?`).
		WithArgs("user1", "manga1", "ch1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock CreateUserNotificationTx calls for user1
	mock.ExpectExec(`INSERT INTO user_notifications`).
		WithArgs("user1", "manga1", "ch1", "New chapters available: Chapter 1, Chapter 2", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock the exists query (no recent notification) for user2
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_notifications WHERE user_name = \? AND media_slug = \? AND chapter_slug = \? AND created_at > \?`).
		WithArgs("user2", "manga1", "ch1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock CreateUserNotificationTx calls for user2
	mock.ExpectExec(`INSERT INTO user_notifications`).
		WithArgs("user2", "manga1", "ch1", "New chapters available: Chapter 1, Chapter 2", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(2, 1))

	// Mock BundleNotificationsForUserTx calls
	mock.ExpectQuery(`SELECT media_slug, COUNT\(\*\) as count FROM user_notifications WHERE user_name = \? AND is_read = 0 GROUP BY media_slug HAVING COUNT\(\*\) > 1`).
		WithArgs("user1").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "count"}))
	mock.ExpectQuery(`SELECT media_slug, COUNT\(\*\) as count FROM user_notifications WHERE user_name = \? AND is_read = 0 GROUP BY media_slug HAVING COUNT\(\*\) > 1`).
		WithArgs("user2").
		WillReturnRows(sqlmock.NewRows([]string{"media_slug", "count"}))

	mock.ExpectCommit()

	err = NotifyUsersOfNewChapters("manga1", []string{"ch1", "ch2"})
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBundleNotificationsForUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock finding media with multiple unread notifications
	mediaRows := sqlmock.NewRows([]string{"media_slug", "count"}).
		AddRow("manga1", 2)
	mock.ExpectQuery(`SELECT media_slug, COUNT\(\*\) as count FROM user_notifications WHERE user_name = \? AND is_read = 0 GROUP BY media_slug HAVING COUNT\(\*\) > 1`).
		WithArgs("user1").
		WillReturnRows(mediaRows)

	// Mock getting notification details
	notifRows := sqlmock.NewRows([]string{"id", "chapter_slug", "message"}).
		AddRow(1, "ch1", "New chapter available: Chapter 1").
		AddRow(2, "ch2", "New chapter available: Chapter 2")
	mock.ExpectQuery(`SELECT id, chapter_slug, message FROM user_notifications WHERE user_name = \? AND media_slug = \? AND is_read = 0 ORDER BY created_at ASC`).
		WithArgs("user1", "manga1").
		WillReturnRows(notifRows)

	// Mock getting chapter names
	chapRows := sqlmock.NewRows([]string{"name"}).
		AddRow("Chapter 1").
		AddRow("Chapter 2")
	mock.ExpectQuery(`SELECT name FROM chapters WHERE media_slug = \? AND slug IN .*`).
		WithArgs("manga1", "ch1", "ch2").
		WillReturnRows(chapRows)

	// Mock deleting old notifications
	mock.ExpectExec(`DELETE FROM user_notifications WHERE id IN .*`).
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Mock creating bundled notification
	mock.ExpectExec(`INSERT INTO user_notifications`).
		WithArgs("user1", "manga1", "ch1", "New chapters available: Chapter 1, Chapter 2", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(3, 1))

	err = BundleNotificationsForUser("user1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBundleNotificationsForUserTx(t *testing.T) {
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

	// Mock finding media with multiple unread notifications
	mediaRows := sqlmock.NewRows([]string{"media_slug", "count"}).
		AddRow("manga1", 2)
	mock.ExpectQuery(`SELECT media_slug, COUNT\(\*\) as count FROM user_notifications WHERE user_name = \? AND is_read = 0 GROUP BY media_slug HAVING COUNT\(\*\) > 1`).
		WithArgs("user1").
		WillReturnRows(mediaRows)

	// Mock getting notification details
	notifRows := sqlmock.NewRows([]string{"id", "chapter_slug", "message"}).
		AddRow(1, "ch1", "New chapter available: Chapter 1").
		AddRow(2, "ch2", "New chapter available: Chapter 2")
	mock.ExpectQuery(`SELECT id, chapter_slug, message FROM user_notifications WHERE user_name = \? AND media_slug = \? AND is_read = 0 ORDER BY created_at ASC`).
		WithArgs("user1", "manga1").
		WillReturnRows(notifRows)

	// Mock getting chapter names
	chapRows := sqlmock.NewRows([]string{"name"}).
		AddRow("Chapter 1").
		AddRow("Chapter 2")
	mock.ExpectQuery(`SELECT name FROM chapters WHERE media_slug = \? AND slug IN .*`).
		WithArgs("manga1", "ch1", "ch2").
		WillReturnRows(chapRows)

	// Mock deleting old notifications
	mock.ExpectExec(`DELETE FROM user_notifications WHERE id IN .*`).
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Mock creating bundled notification
	mock.ExpectExec(`INSERT INTO user_notifications`).
		WithArgs("user1", "manga1", "ch1", "New chapters available: Chapter 1, Chapter 2", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(3, 1))

	err = BundleNotificationsForUserTx(tx, "user1")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAddListener(t *testing.T) {
	// Save original registry
	originalRegistry := ListenerRegistry
	defer func() { ListenerRegistry = originalRegistry }()

	// Clear registry
	ListenerRegistry = []Listener{}

	// Create a mock listener
	mockListener := &MockListener{}

	// Add listener
	AddListener(mockListener)

	// Check that listener was added
	assert.Len(t, ListenerRegistry, 1)
	assert.Equal(t, mockListener, ListenerRegistry[0])
}

func TestNotifyListeners(t *testing.T) {
	// Save original registry
	originalRegistry := ListenerRegistry
	defer func() { ListenerRegistry = originalRegistry }()

	// Clear registry
	ListenerRegistry = []Listener{}

	// Create a mock listener
	mockListener := &MockListener{}

	// Add listener
	ListenerRegistry = append(ListenerRegistry, mockListener)

	// Create a notification
	notification := Notification{
		Type:    "test",
		Payload: "test_payload",
	}

	// Notify listeners
	NotifyListeners(notification)

	// Check that the listener was notified
	assert.True(t, mockListener.Notified)
	assert.Equal(t, notification, mockListener.ReceivedNotification)
}

// MockListener is a mock implementation of the Listener interface for testing
type MockListener struct {
	Notified             bool
	ReceivedNotification Notification
}

func (m *MockListener) Notify(notification Notification) {
	m.Notified = true
	m.ReceivedNotification = notification
}
