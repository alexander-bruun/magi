package email

import (
	"bufio"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/log"
)

const blocklistURL = "https://raw.githubusercontent.com/disposable-email-domains/disposable-email-domains/main/disposable_email_blocklist.conf"

var (
	blockedDomains map[string]struct{}
	once           sync.Once
)

// InitBlocklist fetches the disposable email domain blocklist and caches it.
// Safe to call multiple times; only the first call performs the fetch.
func InitBlocklist() {
	once.Do(func() {
		blockedDomains = make(map[string]struct{})

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(blocklistURL)
		if err != nil {
			log.Warnf("Failed to fetch disposable email blocklist: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Warnf("Disposable email blocklist returned status %d", resp.StatusCode)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		count := 0
		for scanner.Scan() {
			domain := strings.TrimSpace(scanner.Text())
			if domain != "" && !strings.HasPrefix(domain, "#") {
				blockedDomains[strings.ToLower(domain)] = struct{}{}
				count++
			}
		}

		if err := scanner.Err(); err != nil {
			log.Warnf("Error reading disposable email blocklist: %v", err)
		}

		log.Debugf("Loaded %d disposable email domains into blocklist", count)
	})
}

// IsDisposableEmail checks whether the given email address uses a disposable domain.
func IsDisposableEmail(email string) bool {
	if blockedDomains == nil {
		return false
	}

	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}

	domain := strings.ToLower(strings.TrimSpace(parts[1]))
	_, blocked := blockedDomains[domain]
	return blocked
}
