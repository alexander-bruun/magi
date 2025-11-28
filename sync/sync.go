package sync

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2/log"
)

// SyncProvider interface for external service sync
type SyncProvider interface {
	Name() string
	RequiresAuth() bool
	SetAuthToken(token string)
	SyncReadingProgress(userName string, mediaSlug string, chapterSlug string) error
}

// providers map
var providers = make(map[string]func(string) SyncProvider)

// RegisterProvider registers a sync provider
func RegisterProvider(name string, constructor func(string) SyncProvider) {
	providers[name] = constructor
}

// GetProvider returns a sync provider by name
func GetProvider(name string, token string) SyncProvider {
	constructor, exists := providers[name]
	if !exists {
		return nil
	}
	return constructor(token)
}

// SyncReadingProgressForUser syncs progress for all connected services
func SyncReadingProgressForUser(userName string, mediaSlug string, chapterSlug string) {
	// Get all external accounts for the user
	accounts, err := getUserExternalAccounts(userName)
	if err != nil {
		// Log error but don't fail
		return
	}

	for _, account := range accounts {
		provider := GetProvider(account.ServiceName, account.AccessToken)
		if provider != nil {
			go func(p SyncProvider) {
				err := p.SyncReadingProgress(userName, mediaSlug, chapterSlug)
				if err != nil {
					log.Errorf("Sync error for %s: %v", account.ServiceName, err)
				}
			}(provider)
		}
	}
}

// getUserExternalAccounts retrieves all external accounts for a user
func getUserExternalAccounts(userName string) ([]models.UserExternalAccount, error) {
	query := `
	SELECT id, user_name, service_name, external_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at
	FROM user_external_accounts
	WHERE user_name = ?
	`

	rows, err := models.GetDB().Query(query, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.UserExternalAccount
	for rows.Next() {
		var account models.UserExternalAccount
		err := rows.Scan(&account.ID, &account.UserName, &account.ServiceName, &account.ExternalUserID,
			&account.AccessToken, &account.RefreshToken, &account.TokenExpiresAt, &account.CreatedAt, &account.UpdatedAt)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func parseChapterNumber(chapterSlug string) (int, error) {
	// Assuming chapter slug is like "chapter-1" or "ch-001"
	parts := strings.Split(chapterSlug, "-")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid chapter slug")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func parseVolumeNumber(chapterName string) (int, error) {
	// Try to parse volume from chapter name, e.g. "Vol. 1 Ch. 5" or "Volume 1 Chapter 5"
	// Look for patterns like "Vol. X", "Volume X", "V X"
	patterns := []string{
		`Vol\. (\d+)`,
		`Volume (\d+)`,
		`V (\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(chapterName)
		if len(matches) > 1 {
			return strconv.Atoi(matches[1])
		}
	}

	return 0, fmt.Errorf("no volume found in chapter name")
}