package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// Notification represents a notification object
type Notification struct {
	Type    string      // Type of notification (e.g., "library_created", "library_updated", etc.)
	Payload interface{} // Data associated with the notification
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
	MangaSlug   string    `json:"manga_slug"`
	MangaName   string    `json:"manga_name,omitempty"`
	ChapterSlug string    `json:"chapter_slug"`
	ChapterName string    `json:"chapter_name,omitempty"`
	Message     string    `json:"message"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateUserNotification creates a new notification for a user about a new chapter
func CreateUserNotification(userName, mangaSlug, chapterSlug, message string) error {
	query := `
	INSERT INTO user_notifications (user_name, manga_slug, chapter_slug, message, is_read, created_at)
	VALUES (?, ?, ?, ?, 0, ?)
	`

	createdAt := time.Now().Unix()
	_, err := db.Exec(query, userName, mangaSlug, chapterSlug, message, createdAt)
	return err
}

// GetUserNotifications retrieves all notifications for a user, optionally filtered by read status
func GetUserNotifications(userName string, unreadOnly bool) ([]UserNotification, error) {
	query := `
	SELECT n.id, n.user_name, n.manga_slug, n.chapter_slug, n.message, n.is_read, n.created_at,
	       m.name as manga_name, c.name as chapter_name
	FROM user_notifications n
	LEFT JOIN mangas m ON n.manga_slug = m.slug
	LEFT JOIN chapters c ON n.chapter_slug = c.slug AND n.manga_slug = c.manga_slug
	WHERE n.user_name = ?
	`
	
	if unreadOnly {
		query += " AND n.is_read = 0"
	}
	
	query += " ORDER BY n.created_at DESC LIMIT 50"

	rows, err := db.Query(query, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []UserNotification
	for rows.Next() {
		var n UserNotification
		var createdAt int64
		var mangaName, chapterName *string

		if err := rows.Scan(&n.ID, &n.UserName, &n.MangaSlug, &n.ChapterSlug, &n.Message, &n.IsRead, &createdAt, &mangaName, &chapterName); err != nil {
			return nil, err
		}

		n.CreatedAt = time.Unix(createdAt, 0)
		if mangaName != nil {
			n.MangaName = *mangaName
		}
		if chapterName != nil {
			n.ChapterName = *chapterName
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

// NotifyUsersOfNewChapters creates notifications for users reading a manga when new chapters are added
func NotifyUsersOfNewChapters(mangaSlug string, newChapterSlugs []string) error {
	if len(newChapterSlugs) == 0 {
		return nil
	}

	log.Debugf("NotifyUsersOfNewChapters: manga=%s, newChapters=%v", mangaSlug, newChapterSlugs)

	// Get chapter details for the new chapters
	placeholders := make([]string, len(newChapterSlugs))
	args := make([]interface{}, len(newChapterSlugs)+1)
	args[0] = mangaSlug
	for i, slug := range newChapterSlugs {
		placeholders[i] = "?"
		args[i+1] = slug
	}

	query := fmt.Sprintf(`
	SELECT c.slug, c.name
	FROM chapters c
	WHERE c.manga_slug = ? AND c.slug IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Errorf("Failed to query chapters: %v", err)
		return err
	}
	defer rows.Close()

	type chapterInfo struct {
		slug string
		name string
	}

	var newChapters []chapterInfo
	for rows.Next() {
		var ch chapterInfo
		if err := rows.Scan(&ch.slug, &ch.name); err != nil {
			continue
		}
		newChapters = append(newChapters, ch)
	}

	log.Debugf("Found %d chapters for manga '%s'", len(newChapters), mangaSlug)

	if len(newChapters) == 0 {
		return nil
	}

	// Get users who are reading this manga
	usersQuery := `
	SELECT DISTINCT user_name
	FROM reading_states
	WHERE manga_slug = ?
	`

	userRows, err := db.Query(usersQuery, mangaSlug)
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

	log.Debugf("Found %d users reading manga '%s'", len(users), mangaSlug)

	if len(users) == 0 {
		log.Debugf("No users reading manga '%s', skipping notifications", mangaSlug)
		return nil
	}

	// Get manga name
	manga, err := GetMangaUnfiltered(mangaSlug)
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
		message = "New chapter available: " + chapterNames[0]
	} else if len(chapterNames) <= 5 {
		message = "New chapters available: " + strings.Join(chapterNames, ", ")
	} else {
		message = fmt.Sprintf("New chapters available: %s to %s (%d total)", chapterNames[0], chapterNames[len(chapterNames)-1], len(chapterNames))
	}
	firstChapterSlug := newChapters[0].slug

	for _, user := range users {
		// Check if a bundled notification already exists for this manga recently (within last hour)
		existsQuery := `
		SELECT COUNT(*) FROM user_notifications 
		WHERE user_name = ? AND manga_slug = ? AND chapter_slug = ? AND created_at > ?
		`
		var count int
		oneHourAgo := time.Now().Add(-time.Hour).Unix()
		if err := db.QueryRow(existsQuery, user, mangaSlug, firstChapterSlug, oneHourAgo).Scan(&count); err == nil && count > 0 {
			log.Debugf("Recent bundled notification already exists for user %s, manga %s", user, mangaSlug)
			continue // Skip if recent notification exists
		}

		if err := CreateUserNotification(user, mangaSlug, firstChapterSlug, message); err != nil {
			log.Errorf("Failed to create notification: %v", err)
			continue
		}
		notificationCount++
	}

	log.Debugf("Created %d notifications for manga '%s'", notificationCount, mangaSlug)

	// Bundle any existing separate notifications for the same manga
	for _, user := range users {
		if err := BundleNotificationsForUser(user); err != nil {
			log.Errorf("Failed to bundle notifications for user %s: %v", user, err)
		}
	}

	return nil
}

// BundleNotificationsForUser bundles multiple unread notifications for the same manga into one
func BundleNotificationsForUser(userName string) error {
	// Find mangas with multiple unread notifications
	query := `
	SELECT manga_slug, COUNT(*) as count
	FROM user_notifications
	WHERE user_name = ? AND is_read = 0
	GROUP BY manga_slug
	HAVING COUNT(*) > 1
	`

	rows, err := db.Query(query, userName)
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

		// Get all unread notifications for this manga
		notifsQuery := `
		SELECT id, chapter_slug, message
		FROM user_notifications
		WHERE user_name = ? AND manga_slug = ? AND is_read = 0
		ORDER BY created_at ASC
		`
		notifRows, err := db.Query(notifsQuery, userName, mangaSlug)
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
		args := make([]interface{}, len(chapterSlugs)+1)
		args[0] = mangaSlug
		for i, slug := range chapterSlugs {
			placeholders[i] = "?"
			args[i+1] = slug
		}
		chapQuery := fmt.Sprintf(`
		SELECT name FROM chapters WHERE manga_slug = ? AND slug IN (%s)
		`, strings.Join(placeholders, ","))
		chapRows, err := db.Query(chapQuery, args...)
		if err != nil {
			continue
		}
		var chapterNames []string
		for chapRows.Next() {
			var name string
			if err := chapRows.Scan(&name); err != nil {
				continue
			}
			chapterNames = append(chapterNames, name)
		}
		chapRows.Close()

		// Create bundled message
		var message string
		if len(chapterNames) == 1 {
			message = "New chapter available: " + chapterNames[0]
		} else if len(chapterNames) <= 5 {
			message = "New chapters available: " + strings.Join(chapterNames, ", ")
		} else {
			message = fmt.Sprintf("New chapters available: %s to %s (%d total)", chapterNames[0], chapterNames[len(chapterNames)-1], len(chapterNames))
		}

		// Delete old notifications
		deleteQuery := `DELETE FROM user_notifications WHERE id IN (` + strings.Repeat("?,", len(ids)-1) + "?)"
		deleteArgs := make([]interface{}, len(ids))
		for i, id := range ids {
			deleteArgs[i] = id
		}
		_, err = db.Exec(deleteQuery, deleteArgs...)
		if err != nil {
			log.Errorf("Failed to delete old notifications: %v", err)
			continue
		}

		// Create new bundled notification
		if err := CreateUserNotification(userName, mangaSlug, chapterSlugs[0], message); err != nil {
			log.Errorf("Failed to create bundled notification: %v", err)
		}
	}

	return nil
}
