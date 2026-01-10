package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexander-bruun/magi/models"
	"github.com/gofiber/fiber/v2/log"
)

const anilistGraphQLURL = "https://graphql.anilist.co"

// AniListProvider implements the SyncProvider interface for AniList
type AniListProvider struct {
	accessToken string
}

// NewAniListProvider creates a new AniList sync provider
func NewAniListProvider(accessToken string) SyncProvider {
	return &AniListProvider{accessToken: accessToken}
}

func init() {
	RegisterProvider("anilist", NewAniListProvider)
}

func (a *AniListProvider) Name() string {
	return "anilist"
}

func (a *AniListProvider) RequiresAuth() bool {
	return true
}

func (a *AniListProvider) SetAuthToken(token string) {
	a.accessToken = token
}

func (a *AniListProvider) SyncReadingProgress(userName string, mediaSlug string, librarySlug string, chapterSlug string) error {
	// Check if account exists
	account, err := models.GetUserExternalAccount(userName, "anilist")
	if err != nil {
		log.Warnf("No AniList account for user %s", userName)
		return err
	}

	// Get all read chapters for this user and media
	readChapters, err := models.GetReadChaptersForUser(userName, mediaSlug)
	if err != nil {
		log.Errorf("AniList sync: failed to get read chapters for user %s, media %s: %v", userName, mediaSlug, err)
		return err
	}

	// Find the highest chapter number read
	maxChapterNum := 0
	for chapterSlug := range readChapters {
		chapterNum, err := parseChapterNumber(chapterSlug)
		if err != nil {
			log.Debugf("AniList sync: failed to parse chapter number from %s: %v", chapterSlug, err)
			continue
		}
		if chapterNum > maxChapterNum {
			maxChapterNum = chapterNum
		}
	}

	if maxChapterNum == 0 {
		log.Debugf("AniList sync: no chapters read yet for user %s, media %s", userName, mediaSlug)
		return nil // Nothing to sync
	}

	log.Debugf("AniList sync: user has read up to chapter %d", maxChapterNum)

	// Parse volume number from the current chapter (if available)
	chapter, err := models.GetChapter(mediaSlug, librarySlug, chapterSlug)
	volumeNum := 0
	if err == nil {
		volumeNum, err = parseVolumeNumber(chapter.Name)
		if err != nil {
			// If volume can't be parsed, set to 0 or current
			volumeNum = 0
		}
	}

	// Find the manga on AniList
	anilistMangaID, err := a.findMangaOnAniList(mediaSlug)
	if err != nil {
		log.Errorf("Failed to find manga %s on AniList", mediaSlug)
		return err
	}

	// Update progress
	err = a.updateProgress(account.AccessToken, anilistMangaID, maxChapterNum, volumeNum)
	if err != nil {
		log.Errorf("AniList sync: failed to update progress: %v", err)
		return err
	}

	log.Debugf("Synced series: %s (AniList): success", mediaSlug)
	return nil
}

func (a *AniListProvider) findMangaOnAniList(mediaSlug string) (int, error) {
	// Search for the manga by title (assuming slug is based on title)
	title := strings.ReplaceAll(strings.ReplaceAll(mediaSlug, "-", " "), "/", " ")

	query := `
	query ($search: String) {
		Media(search: $search, type: MANGA) {
			id
		}
	}`

	variables := map[string]any{
		"search": title,
	}

	requestBody := map[string]any{
		"query":     query,
		"variables": variables,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", anilistGraphQLURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		Data struct {
			Media struct {
				ID int `json:"id"`
			} `json:"Media"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if result.Data.Media.ID == 0 {
		return 0, fmt.Errorf("no manga found")
	}

	return result.Data.Media.ID, nil
}

func (a *AniListProvider) updateProgress(accessToken string, mangaID int, chapterNum int, volumeNum int) error {
	log.Debugf("AniList updateProgress: updating manga %d with chapters %d, volumes %d", mangaID, chapterNum, volumeNum)
	mutation := `
	mutation ($mediaId: Int, $progress: Int, $progressVolumes: Int) {
		SaveMediaListEntry(mediaId: $mediaId, progress: $progress, progressVolumes: $progressVolumes) {
			id
			progress
			progressVolumes
		}
	}`

	variables := map[string]any{
		"mediaId":         mangaID,
		"progress":        chapterNum,
		"progressVolumes": volumeNum,
	}

	requestBody := map[string]any{
		"query":     mutation,
		"variables": variables,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("AniList updateProgress: marshal error: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", anilistGraphQLURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("AniList updateProgress: request error: %v", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("AniList updateProgress: request error: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("AniList updateProgress: failed with status %d", resp.StatusCode)
		return fmt.Errorf("update failed with status %d", resp.StatusCode)
	}

	log.Debug("AniList updateProgress: success")
	return nil
}
