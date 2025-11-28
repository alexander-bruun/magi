package sync

import (
	"github.com/alexander-bruun/magi/models"
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
				_ = p.SyncReadingProgress(userName, mediaSlug, chapterSlug)
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