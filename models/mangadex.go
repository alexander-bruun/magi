package models

// MangaResponse represents the structure of the JSON response from MangaDex API
type MangaResponse struct {
	Result string        `json:"result"`
	Data   []MangaDetail `json:"data"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	Total  int           `json:"total"`
}

// MangaDetail represents details of a manga item in the "data" array of MangaResponse
type MangaDetail struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Attributes    MangaAttributes `json:"attributes"`
	Relationships []Relationship  `json:"relationships"`
}

// MangaAttributes represents the attributes of a manga in MangaDetail
type MangaAttributes struct {
	Title                  map[string]string   `json:"title"`
	AltTitles              []map[string]string `json:"altTitles"`
	Description            map[string]string   `json:"description"`
	IsLocked               bool                `json:"isLocked"`
	Links                  map[string]string   `json:"links"`
	OriginalLanguage       string              `json:"originalLanguage"`
	LastVolume             string              `json:"lastVolume"`
	LastChapter            string              `json:"lastChapter"`
	PublicationDemographic interface{}         `json:"publicationDemographic"`
	Status                 string              `json:"status"`
	Year                   int                 `json:"year"`
	ContentRating          string              `json:"contentRating"`
	Tags                   []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Name        map[string]string `json:"name"`
			Description map[string]string `json:"description"`
			Group       string            `json:"group"`
			Version     int               `json:"version"`
		} `json:"attributes"`
		Relationships []interface{} `json:"relationships"`
	} `json:"tags"`
	State                          string   `json:"state"`
	ChapterNumbersResetOnNewVolume bool     `json:"chapterNumbersResetOnNewVolume"`
	CreatedAt                      string   `json:"createdAt"`
	UpdatedAt                      string   `json:"updatedAt"`
	Version                        int      `json:"version"`
	AvailableTranslatedLanguages   []string `json:"availableTranslatedLanguages"`
	LatestUploadedChapter          string   `json:"latestUploadedChapter"`
}

// Relationship represents the relationship details in MangaDetail
type Relationship struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		FileName string `json:"fileName"`
	} `json:"attributes"`
}
