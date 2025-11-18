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

	log.Infof("NotifyUsersOfNewChapters: manga=%s, newChapters=%v", mangaSlug, newChapterSlugs)

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

	log.Infof("Found %d chapter details for manga '%s'", len(newChapters), mangaSlug)

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

	log.Infof("Found %d users reading manga '%s'", len(users), mangaSlug)

	if len(users) == 0 {
		log.Infof("No users reading manga '%s', skipping notifications", mangaSlug)
		return nil
	}

	// Get manga name
	manga, err := GetMangaUnfiltered(mangaSlug)
	if err != nil || manga == nil {
		log.Errorf("Failed to get manga details: %v", err)
		return err
	}

	// Create notifications for each user for each new chapter
	notificationCount := 0
	for _, user := range users {
		for _, chapter := range newChapters {
			// Check if notification already exists to avoid duplicates
			existsQuery := `
			SELECT COUNT(*) FROM user_notifications 
			WHERE user_name = ? AND manga_slug = ? AND chapter_slug = ?
			`
			var count int
			if err := db.QueryRow(existsQuery, user, mangaSlug, chapter.slug).Scan(&count); err == nil && count > 0 {
				log.Debugf("Notification already exists for user %s, manga %s, chapter %s", user, mangaSlug, chapter.slug)
				continue // Skip if notification already exists
			}

			message := "New chapter available: " + chapter.name
			if err := CreateUserNotification(user, mangaSlug, chapter.slug, message); err != nil {
				log.Errorf("Failed to create notification: %v", err)
				continue
			}
			notificationCount++
		}
	}

	log.Infof("Created %d notifications for manga '%s'", notificationCount, mangaSlug)
	return nil
}

// GetRecentlyAddedChapters returns chapters added after the specified time for a manga
func GetRecentlyAddedChapters(mangaSlug string, afterTime time.Time) ([]Chapter, error) {
	// Note: chapters table doesn't have created_at, so we'll need to check all chapters
	// and compare against existing notifications to determine which are "new"
	return GetChapters(mangaSlug)
}
