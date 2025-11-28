package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2/log"
)

const malBaseURL = "https://api.myanimelist.net/v2"

// MALProvider implements the SyncProvider interface for MyAnimeList
type MALProvider struct {
	apiToken string
}

// NewMALProvider creates a new MAL sync provider
func NewMALProvider(apiToken string) SyncProvider {
	return &MALProvider{apiToken: apiToken}
}

func init() {
	RegisterProvider("mal", NewMALProvider)
}

func (m *MALProvider) Name() string {
	return "mal"
}

func (m *MALProvider) RequiresAuth() bool {
	return true
}

func (m *MALProvider) SetAuthToken(token string) {
	m.apiToken = token
}

func (m *MALProvider) SyncReadingProgress(userName string, mediaSlug string, chapterSlug string) error {
	// Check if account exists
	_, err := models.GetUserExternalAccount(userName, "mal")
	if err != nil {
		log.Warn(fmt.Sprintf("No MAL account for user %s", userName))
		return err
	}

	// Parse chapter number from slug
	chapterNum, err := parseChapterNumber(chapterSlug)
	if err != nil {
		log.Warn(fmt.Sprintf("Failed to parse chapter number from %s", chapterSlug))
		return err
	}

	// Find the manga on MAL
	malMangaID, err := m.findMangaOnMAL(mediaSlug)
	if err != nil {
		log.Warn(fmt.Sprintf("Failed to find manga %s on MAL", mediaSlug))
		return err
	}

	// Update progress
	return m.updateProgress(malMangaID, chapterNum)
}

func (m *MALProvider) findMangaOnMAL(mediaSlug string) (int, error) {
	// Search for the manga by title (assuming slug is based on title)
	// Replace hyphens and slashes with spaces for search
	title := strings.ReplaceAll(strings.ReplaceAll(mediaSlug, "-", " "), "/", " ")
	encodedTitle := url.QueryEscape(title)
	url := fmt.Sprintf("%s/manga?q=%s&limit=1", malBaseURL, encodedTitle)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Node struct {
				ID int `json:"id"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Data) == 0 {
		return 0, fmt.Errorf("no manga found")
	}

	return result.Data[0].Node.ID, nil
}

func (m *MALProvider) updateProgress(mangaID int, chapterNum int) error {
	url := fmt.Sprintf("%s/manga/%d/my_list_status", malBaseURL, mangaID)
	data := map[string]interface{}{
		"num_chapters_read": chapterNum,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update failed with status %d", resp.StatusCode)
	}

	return nil
}

func parseChapterNumber(chapterSlug string) (int, error) {
	// Assuming chapter slug is like "chapter-1" or "ch-001"
	parts := strings.Split(chapterSlug, "-")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid chapter slug")
	}
	return strconv.Atoi(parts[len(parts)-1])
}