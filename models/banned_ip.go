package models

import (
	"time"

	"github.com/gofiber/fiber/v2/log"
)

// BannedIP represents a banned IP address
type BannedIP struct {
	ID        int       `json:"id"`
	IPAddress string    `json:"ip_address"`
	BannedAt  time.Time `json:"banned_at"`
	Reason    string    `json:"reason"`
}

// BanIP adds an IP to the banned list
func BanIP(ip, reason string) error {
	_, err := db.Exec(`
		INSERT INTO banned_ips (ip_address, reason)
		VALUES (?, ?)
	`, ip, reason)
	if err != nil {
		log.Errorf("Failed to ban IP %s: %v", ip, err)
	}
	return err
}

// UnbanIP removes an IP from the banned list
func UnbanIP(ip string) error {
	_, err := db.Exec(`
		DELETE FROM banned_ips WHERE ip_address = ?
	`, ip)
	return err
}

// IsIPBanned checks if an IP is banned
func IsIPBanned(ip string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM banned_ips WHERE ip_address = ?
	`, ip).Scan(&count)
	if err != nil {
		log.Errorf("Failed to check if IP %s is banned: %v", ip, err)
		return false, err
	}
	banned := count > 0
	if banned {
		log.Infof("IP %s is banned", ip)
	}
	return banned, nil
}

// GetBannedIPs retrieves all banned IPs
func GetBannedIPs() ([]BannedIP, error) {
	rows, err := db.Query(`
		SELECT id, ip_address, banned_at, reason
		FROM banned_ips
		ORDER BY banned_at DESC
	`)
	if err != nil {
		log.Errorf("Failed to query banned IPs: %v", err)
		return nil, err
	}
	defer rows.Close()

	var bannedIPs []BannedIP
	for rows.Next() {
		var bip BannedIP
		err := rows.Scan(&bip.ID, &bip.IPAddress, &bip.BannedAt, &bip.Reason)
		if err != nil {
			log.Errorf("Failed to scan banned IP: %v", err)
			return nil, err
		}
		bannedIPs = append(bannedIPs, bip)
	}
	log.Debugf("Retrieved %d banned IPs", len(bannedIPs))
	return bannedIPs, nil
}