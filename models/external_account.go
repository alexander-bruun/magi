package models

import (
	"time"
)

// UserExternalAccount represents external service accounts linked to users
type UserExternalAccount struct {
	ID             int64     `json:"id"`
	UserName       string    `json:"user_name"`
	ServiceName    string    `json:"service_name"`
	ExternalUserID string    `json:"external_user_id"`
	AccessToken    string    `json:"access_token"`
	RefreshToken   string    `json:"refresh_token"`
	TokenExpiresAt time.Time `json:"token_expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GetUserExternalAccount retrieves an external account for a user and service
func GetUserExternalAccount(userName, serviceName string) (*UserExternalAccount, error) {
	query := `
	SELECT id, user_name, service_name, external_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at
	FROM user_external_accounts
	WHERE user_name = ? AND service_name = ?
	`

	var account UserExternalAccount
	row := db.QueryRow(query, userName, serviceName)
	err := row.Scan(&account.ID, &account.UserName, &account.ServiceName, &account.ExternalUserID,
		&account.AccessToken, &account.RefreshToken, &account.TokenExpiresAt, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &account, nil
}

// SaveUserExternalAccount inserts or updates an external account
func SaveUserExternalAccount(account *UserExternalAccount) error {
	query := `
	INSERT INTO user_external_accounts (user_name, service_name, external_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_name, service_name) DO UPDATE SET
		external_user_id = excluded.external_user_id,
		access_token = excluded.access_token,
		refresh_token = excluded.refresh_token,
		token_expires_at = excluded.token_expires_at,
		updated_at = excluded.updated_at
	`

	_, err := db.Exec(query, account.UserName, account.ServiceName, account.ExternalUserID,
		account.AccessToken, account.RefreshToken, account.TokenExpiresAt, account.CreatedAt, account.UpdatedAt)
	return err
}

// DeleteUserExternalAccount removes an external account
func DeleteUserExternalAccount(userName, serviceName string) error {
	query := `
	DELETE FROM user_external_accounts
	WHERE user_name = ? AND service_name = ?
	`

	_, err := db.Exec(query, userName, serviceName)
	return err
}

// GetUserExternalAccounts retrieves all external accounts for a user
func GetUserExternalAccounts(userName string) ([]UserExternalAccount, error) {
	query := `
	SELECT id, user_name, service_name, external_user_id, access_token, refresh_token, token_expires_at, created_at, updated_at
	FROM user_external_accounts
	WHERE user_name = ?
	`

	rows, err := db.Query(query, userName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []UserExternalAccount
	for rows.Next() {
		var account UserExternalAccount
		err := rows.Scan(&account.ID, &account.UserName, &account.ServiceName, &account.ExternalUserID,
			&account.AccessToken, &account.RefreshToken, &account.TokenExpiresAt, &account.CreatedAt, &account.UpdatedAt)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}
