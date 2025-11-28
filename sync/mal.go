package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
	log.Debugf("MAL sync: updating status only (chapter progress ignored due to API limitations)")
	// MAL API v2 accepts updates but doesn't properly save num_read_chapters.
	// We can still update other fields like status, but skip chapter progress for now.

	// Check if account exists
	_, err := models.GetUserExternalAccount(userName, "mal")
	if err != nil {
		log.Warnf("MAL sync: no account for user %s: %v", userName, err)
		return err
	}

	// Find the manga on MAL
	malMangaID, err := m.findMangaOnMAL(mediaSlug)
	if err != nil {
		log.Errorf("MAL sync: failed to find manga %s on MAL: %v", mediaSlug, err)
		return err
	}

	log.Debugf("MAL sync: found MAL ID %d", malMangaID)

	// Update status only (no chapter progress)
	err = m.updateStatusOnly(malMangaID)
	if err != nil {
		log.Errorf("MAL sync: failed to update status: %v", err)
		return err
	}

	log.Info("MAL sync: status update completed")
	return nil
}

// findMangaOnMAL is currently unused - MAL API v2 doesn't properly update progress
func (m *MALProvider) findMangaOnMAL(mediaSlug string) (int, error) {
	// Get the actual media name from the database
	media, err := models.GetMedia(mediaSlug)
	if err != nil {
		return 0, fmt.Errorf("failed to get media %s: %v", mediaSlug, err)
	}

	log.Infof("MAL sync: searching for manga with title '%s'", media.Name)
	// Search for the manga by title
	encodedTitle := url.QueryEscape(media.Name)
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

	log.Infof("MAL sync: search returned %d results", len(result.Data))
	if len(result.Data) == 0 {
		return 0, fmt.Errorf("no manga found for title '%s'", media.Name)
	}

	mangaID := result.Data[0].Node.ID
	log.Debugf("Synced series: %s (MAL): success", mediaSlug)
	return mangaID, nil
}

// updateStatusOnly updates MAL status without chapter progress (currently just ensures manga is in reading list)
func (m *MALProvider) updateStatusOnly(mangaID int) error {
	log.Infof("MAL updateStatusOnly: updating manga %d status to reading", mangaID)
	url := fmt.Sprintf("%s/manga/%d/my_list_status", malBaseURL, mangaID)
	data := map[string]interface{}{
		"status": "reading", // Ensure manga is marked as reading
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Errorf("MAL updateStatusOnly: marshal error: %v", err)
		return err
	}

	log.Debugf("MAL updateStatusOnly: sending request to %s with data: %s", url, string(jsonData))
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("MAL updateStatusOnly: request error: %v", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("MAL updateStatusOnly: do error: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Errorf("MAL updateStatusOnly: failed with status %d, body: %s", resp.StatusCode, string(body))
		return fmt.Errorf("status update failed with status %d", resp.StatusCode)
	}

	log.Debug("MAL updateStatusOnly: success")
	return nil
}