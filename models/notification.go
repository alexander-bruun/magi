package models

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/log"
)

// Notification represents a notification object
type Notification struct {
	Type    string // Type of notification (e.g., "library_created", "library_updated", etc.)
	Payload any    // Data associated with the notification
}

// Listener defines the interface for objects that want to listen to notifications
type Listener interface {
	Notify(notification Notification)
}

// ListenerRegistry is a registry of listeners interested in notifications
var ListenerRegistry []Listener

// AddListener adds a listener to the registry
func AddListener(listener Listener) {
	ListenerRegistry = append(ListenerRegistry, listener)
}

// NotifyListeners notifies all registered listeners with a given notification
func NotifyListeners(notification Notification) {
	for _, listener := range ListenerRegistry {
		listener.Notify(notification)
	}
}

// UserNotification represents a persistent user notification about new chapters
type UserNotification struct {
	ID          int64     `json:"id"`
	UserName    string    `json:"user_name"`
	MediaSlug   string    `json:"media_slug"`
	MangaName   string    `json:"manga_name,omitempty"`
	ChapterSlug string    `json:"chapter_slug"`
	ChapterID   string    `json:"chapter_id,omitempty"`
	ChapterName string    `json:"chapter_name,omitempty"`
	Message     string    `json:"message"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
	Type        string    `json:"type"` // "chapter" or "admin_issue"
}

// createUserNotification creates a new notification for a user about a new chapter
func createUserNotification(userName, mangaSlug, chapterSlug, message string) error {
	return createNotificationWith(db, userName, mangaSlug, "", chapterSlug, message, "chapter")
}

// createUserNotificationWithType creates a new notification for a user with a specific type
func createUserNotificationWithType(userName, mediaSlug, chapterSlug, message, notificationType string) error {
	return createNotificationWith(db, userName, mediaSlug, "", chapterSlug, message, notificationType)
}

// CreateAdminNotification creates a new admin notification for moderators/admins
func CreateAdminNotification(userName, message string) error {
	return createNotificationWith(db, userName, "admin", "", "", message, "admin_issue")
}

// createUserNotificationTx creates a new notification for a user about a new chapter within a transaction
func createUserNotificationTx(tx *sql.Tx, userName, mangaSlug, librarySlug, chapterSlug, message string) error {
	return createNotificationWith(tx, userName, mangaSlug, librarySlug, chapterSlug, message, "chapter")
}

// createUserNotificationTxWithType creates a new notification for a user with a specific type within a transaction
func createUserNotificationTxWithType(tx *sql.Tx, userName, mediaSlug, librarySlug, chapterSlug, message, notificationType string) error {
	return createNotificationWith(tx, userName, mediaSlug, librarySlug, chapterSlug, message, notificationType)
}

// createNotificationWith creates a notification using the given Executor.
func createNotificationWith(exec Executor, userName, mediaSlug, librarySlug, chapterSlug, message, notificationType string) error {
	query := `
	INSERT INTO user_notifications (user_name, media_slug, library_slug, chapter_slug, message, is_read, created_at, type)
	VALUES (?, ?, ?, ?, ?, 0, ?, ?)
	`

	createdAt := time.Now().Unix()

	// For admin notifications, use NULL for media/chapter fields
	if notificationType == "admin_issue" {
		_, err := exec.Exec(query, userName, nil, nil, nil, message, createdAt, notificationType)
		return err
	}

	// For chapter notifications, use the provided values
	_, err := exec.Exec(query, userName, mediaSlug, librarySlug, chapterSlug, message, createdAt, notificationType)
	return err
}

// GetUserNotifications retrieves all notifications for a user, optionally filtered by read status
func GetUserNotifications(userName string, unreadOnly bool) ([]UserNotification, error) {
	query := `
	SELECT n.id, n.user_name, n.media_slug, n.chapter_slug, n.message, n.is_read, n.created_at, n.type,
	       COALESCE(m.name, '') as manga_name, COALESCE(c.name, '') as chapter_name, COALESCE(c.id, '') as chapter_id
	FROM user_notifications n
	LEFT JOIN media m ON n.media_slug = m.slug AND n.type = 'chapter'
	LEFT JOIN chapters c ON n.chapter_slug = c.slug AND n.media_slug = c.media_slug AND n.type = 'chapter'
	WHERE n.user_name = ?
	`

	if unreadOnly {
		query += " AND n.is_read = 0"
	}

	query += " ORDER BY n.created_at DESC LIMIT 10"

	rows, err := db.Query(query, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []UserNotification
	for rows.Next() {
		var n UserNotification
		var createdAt int64
		var mangaName, chapterName, chapterID string
		var mediaSlug, chapterSlug sql.NullString

		if err := rows.Scan(&n.ID, &n.UserName, &mediaSlug, &chapterSlug, &n.Message, &n.IsRead, &createdAt, &n.Type, &mangaName, &chapterName, &chapterID); err != nil {
			return nil, err
		}

		n.CreatedAt = time.Unix(createdAt, 0)
		if mediaSlug.Valid {
			n.MediaSlug = mediaSlug.String
		}
		if chapterSlug.Valid {
			n.ChapterSlug = chapterSlug.String
		}
		if n.Type == "chapter" {
			n.MangaName = mangaName
			n.ChapterName = chapterName
			n.ChapterID = chapterID
		}

		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

// GetUnreadNotificationCount returns the count of unread notifications for a user
func GetUnreadNotificationCount(userName string) (int, error) {
	query := `SELECT COUNT(*) FROM user_notifications WHERE user_name = ? AND is_read = 0`

	var count int
	err := db.QueryRow(query, userName).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// MarkNotificationAsRead marks a specific notification as read
func MarkNotificationAsRead(notificationID int64, userName string) error {
	query := `UPDATE user_notifications SET is_read = 1 WHERE id = ? AND user_name = ?`
	_, err := db.Exec(query, notificationID, userName)
	return err
}

// MarkAllNotificationsAsRead marks all notifications for a user as read
func MarkAllNotificationsAsRead(userName string) error {
	query := `UPDATE user_notifications SET is_read = 1 WHERE user_name = ? AND is_read = 0`
	_, err := db.Exec(query, userName)
	return err
}

// DeleteNotification deletes a specific notification
func DeleteNotification(notificationID int64, userName string) error {
	query := `DELETE FROM user_notifications WHERE id = ? AND user_name = ?`
	_, err := db.Exec(query, notificationID, userName)
	return err
}

// ClearReadNotifications deletes all read notifications for a user
func ClearReadNotifications(userName string) error {
	query := `DELETE FROM user_notifications WHERE user_name = ? AND is_read = 1`
	_, err := db.Exec(query, userName)
	return err
}

// DeleteNotificationsForChapter deletes all notifications for a specific chapter
func DeleteNotificationsForChapter(mediaSlug, librarySlug, chapterSlug string) error {
	query := `DELETE FROM user_notifications WHERE media_slug = ? AND library_slug = ? AND chapter_slug = ?`
	_, err := db.Exec(query, mediaSlug, librarySlug, chapterSlug)
	return err
}

// NotifyUsersOfNewChapters creates notifications for users reading a manga when new chapters are added
func NotifyUsersOfNewChapters(mangaSlug string, newChapterSlugs []string) error {
	if len(newChapterSlugs) == 0 {
		return nil
	}

	log.Debugf("NotifyUsersOfNewChapters: manga=%s, newChapters=%v", mangaSlug, newChapterSlugs)

	tx, err := BeginTx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get chapter details for the new chapters
	placeholders := make([]string, len(newChapterSlugs))
	args := make([]any, len(newChapterSlugs)+1)
	args[0] = mangaSlug
	for i, slug := range newChapterSlugs {
		placeholders[i] = "?"
		args[i+1] = slug
	}

	query := fmt.Sprintf(`
	SELECT c.slug, c.name, c.library_slug
	FROM chapters c
	JOIN libraries l ON c.library_slug = l.slug
	WHERE c.media_slug = ? AND c.slug IN (%s) AND l.enabled = true
	`, strings.Join(placeholders, ","))

	rows, err := tx.Query(query, args...)
	if err != nil {
		log.Errorf("Failed to query chapters: %v", err)
		return err
	}
	defer rows.Close()

	type chapterInfo struct {
		slug        string
		name        string
		librarySlug string
	}

	var newChapters []chapterInfo
	for rows.Next() {
		var ch chapterInfo
		if err := rows.Scan(&ch.slug, &ch.name, &ch.librarySlug); err != nil {
			continue
		}
		newChapters = append(newChapters, ch)
	}

	log.Debugf("Found %d chapters for manga '%s'", len(newChapters), mangaSlug)

	if len(newChapters) == 0 {
		return tx.Commit() // Nothing to do, but commit the transaction
	}

	// Sort chapters by chapter number for proper range display
	sort.Slice(newChapters, func(i, j int) bool {
		numI := extractChapterNumber(newChapters[i].name)
		numJ := extractChapterNumber(newChapters[j].name)
		// Handle cases where extraction fails (-1)
		if numI == -1 && numJ == -1 {
			return newChapters[i].name < newChapters[j].name
		}
		if numI == -1 {
			return false // items with no number go to the end
		}
		if numJ == -1 {
			return true // items with no number go to the end
		}
		return numI < numJ
	})

	// Get users who have favorited this manga
	usersQuery := `
	SELECT DISTINCT user_username
	FROM favorites
	WHERE media_slug = ?
	`

	userRows, err := tx.Query(usersQuery, mangaSlug)
	if err != nil {
		log.Errorf("Failed to query users: %v", err)
		return err
	}
	defer userRows.Close()

	var users []string
	for userRows.Next() {
		var userName string
		if err := userRows.Scan(&userName); err != nil {
			continue
		}
		users = append(users, userName)
	}

	log.Debugf("Found %d users who favorited manga '%s'", len(users), mangaSlug)

	if len(users) == 0 {
		log.Debugf("No users have favorited manga '%s', skipping notifications", mangaSlug)
		return tx.Commit()
	}

	// Get manga name
	manga, err := GetMediaUnfiltered(mangaSlug)
	if err != nil || manga == nil {
		log.Errorf("Failed to get manga details: %v", err)
		return err
	}

	// Create notifications for each user with all new chapters bundled
	notificationCount := 0
	chapterNames := make([]string, len(newChapters))
	for i, ch := range newChapters {
		chapterNames[i] = ch.name
	}
	var message string
	if len(chapterNames) == 1 {
		message = fmt.Sprintf("New chapter available for %s: %s", manga.Name, chapterNames[0])
	} else if len(chapterNames) <= 5 {
		message = fmt.Sprintf("New chapters available for %s: %s", manga.Name, strings.Join(chapterNames, ", "))
	} else {
		message = fmt.Sprintf("New chapters available for %s: %s to %s (%d total)", manga.Name, chapterNames[0], chapterNames[len(chapterNames)-1], len(chapterNames))
	}
	firstChapterSlug := newChapters[0].slug

	for _, user := range users {
		// Check if a bundled notification already exists for this manga recently (within last hour)
		existsQuery := `
		SELECT COUNT(*) FROM user_notifications 
		WHERE user_name = ? AND media_slug = ? AND chapter_slug = ? AND created_at > ?
		`
		var count int
		oneHourAgo := time.Now().Add(-time.Hour).Unix()
		if err := tx.QueryRow(existsQuery, user, mangaSlug, firstChapterSlug, oneHourAgo).Scan(&count); err == nil && count > 0 {
			log.Debugf("Recent bundled notification already exists for user %s, manga %s", user, mangaSlug)
			continue // Skip if recent notification exists
		}

		if err := createUserNotificationTx(tx, user, mangaSlug, newChapters[0].librarySlug, firstChapterSlug, message); err != nil {
			log.Errorf("Failed to create notification: %v", err)
			continue
		}
		notificationCount++
	}

	log.Debugf("Created %d notifications for manga '%s'", notificationCount, mangaSlug)

	// Bundle any existing separate notifications for the same manga
	for _, user := range users {
		if err := BundleNotificationsForUserTx(tx, user); err != nil {
			log.Errorf("Failed to bundle notifications for user %s: %v", user, err)
		}
	}

	return tx.Commit()
}

// BundleNotificationsForUser bundles multiple unread notifications for the same manga into one
func BundleNotificationsForUser(userName string) error {
	return bundleNotificationsWith(db, userName)
}

// BundleNotificationsForUserTx bundles multiple unread notifications for the same manga into one within a transaction
func BundleNotificationsForUserTx(tx *sql.Tx, userName string) error {
	return bundleNotificationsWith(tx, userName)
}

// bundleNotificationsWith bundles multiple unread notifications using the given Executor.
func bundleNotificationsWith(exec Executor, userName string) error {
	// Find media with multiple unread notifications
	query := `
	SELECT media_slug, COUNT(*) as count
	FROM user_notifications
	WHERE user_name = ? AND is_read = 0
	GROUP BY media_slug
	HAVING COUNT(*) > 1
	`

	rows, err := exec.Query(query, userName)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var mangaSlug string
		var count int
		if err := rows.Scan(&mangaSlug, &count); err != nil {
			continue
		}

		// Get manga name
		manga, err := GetMediaUnfiltered(mangaSlug)
		if err != nil || manga == nil {
			log.Errorf("Failed to get manga details for bundling: %v", err)
			continue
		}

		// Get all unread notifications for this manga
		notifsQuery := `
		SELECT id, chapter_slug, message
		FROM user_notifications
		WHERE user_name = ? AND media_slug = ? AND is_read = 0
		ORDER BY created_at ASC
		`
		notifRows, err := exec.Query(notifsQuery, userName, mangaSlug)
		if err != nil {
			continue
		}

		var ids []int64
		var chapterSlugs []string
		var messages []string
		for notifRows.Next() {
			var id int64
			var chapterSlug, message string
			if err := notifRows.Scan(&id, &chapterSlug, &message); err != nil {
				continue
			}
			ids = append(ids, id)
			chapterSlugs = append(chapterSlugs, chapterSlug)
			messages = append(messages, message)
		}
		notifRows.Close()

		if len(ids) <= 1 {
			continue
		}

		// Get chapter names
		placeholders := make([]string, len(chapterSlugs))
		args := make([]any, len(chapterSlugs)+1)
		args[0] = mangaSlug
		for i, slug := range chapterSlugs {
			placeholders[i] = "?"
			args[i+1] = slug
		}
		chapQuery := fmt.Sprintf(`
		SELECT name, c.library_slug FROM chapters c JOIN libraries l ON c.library_slug = l.slug WHERE c.media_slug = ? AND c.slug IN (%s) AND l.enabled = true
		`, strings.Join(placeholders, ","))
		chapRows, err := exec.Query(chapQuery, args...)
		if err != nil {
			continue
		}
		var chapterNames []string
		var librarySlug string
		for chapRows.Next() {
			var name, libSlug string
			if err := chapRows.Scan(&name, &libSlug); err != nil {
				continue
			}
			chapterNames = append(chapterNames, name)
			if librarySlug == "" {
				librarySlug = libSlug
			}
		}
		chapRows.Close()

		// Create bundled message
		var message string
		if len(chapterNames) == 1 {
			message = fmt.Sprintf("New chapter available for %s: %s", manga.Name, chapterNames[0])
		} else if len(chapterNames) <= 5 {
			message = fmt.Sprintf("New chapters available for %s: %s", manga.Name, strings.Join(chapterNames, ", "))
		} else {
			message = fmt.Sprintf("New chapters available for %s: %s to %s (%d total)", manga.Name, chapterNames[0], chapterNames[len(chapterNames)-1], len(chapterNames))
		}

		// Delete old notifications
		deleteQuery := `DELETE FROM user_notifications WHERE id IN (` + strings.Repeat("?,", len(ids)-1) + "?)"
		deleteArgs := make([]any, len(ids))
		for i, id := range ids {
			deleteArgs[i] = id
		}
		_, err = exec.Exec(deleteQuery, deleteArgs...)
		if err != nil {
			log.Errorf("Failed to delete old notifications: %v", err)
			continue
		}

		// Create new bundled notification
		if err := createNotificationWith(exec, userName, mangaSlug, librarySlug, chapterSlugs[0], message, "chapter"); err != nil {
			log.Errorf("Failed to create bundled notification: %v", err)
		}
	}

	return nil
}
