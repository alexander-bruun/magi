package models

import (
	"strings"
)

// SearchOptions defines parameters for manga searches
type SearchOptions struct {
	Filter             string
	Page               int
	PageSize           int
	SortBy             string
	SortOrder          string
	FilterBy           string
	LibrarySlug        string
	Tags               []string
	TagMode            string // "all" or "any"
	Types              []string // filter by manga types (any match)
	AccessibleLibraries []string // filter by accessible libraries for permission system
}

// SearchMangasWithOptions performs a flexible manga search using options
func SearchMangasWithOptions(opts SearchOptions) ([]Manga, int64, error) {
	var mangas []Manga
	if err := loadAllMangas(&mangas); err != nil {
		return nil, 0, err
	}

	// Filter by accessible libraries (permission system)
	if len(opts.AccessibleLibraries) > 0 {
		mangas = filterByAccessibleLibraries(mangas, opts.AccessibleLibraries)
	}

	// Filter by library
	if opts.LibrarySlug != "" {
		mangas = filterByLibrarySlug(mangas, opts.LibrarySlug)
	}

	// Filter by tags if provided
	if len(opts.Tags) > 0 {
		tagMap, err := GetAllMangaTagsMap()
		if err != nil {
			return nil, 0, err
		}

		if opts.TagMode == "any" {
			mangas = filterByAnyTag(mangas, opts.Tags, tagMap)
		} else {
			mangas = filterByAllTags(mangas, opts.Tags, tagMap)
		}
	}

	// Filter by types if provided (any match)
	if len(opts.Types) > 0 {
		typeSet := normalizeStringSet(opts.Types)
		filtered := make([]Manga, 0, len(mangas))
		for _, m := range mangas {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(m.Type))]; ok {
				filtered = append(filtered, m)
			}
		}
		mangas = filtered
	}

	total := int64(len(mangas))

	// Apply text search filter
	if opts.Filter != "" {
		mangas = applyBigramSearch(opts.Filter, mangas)
		total = int64(len(mangas))
	}

	// Sort results
	key, ord := MangaSortConfig.NormalizeSort(opts.SortBy, opts.SortOrder)
	SortMangas(mangas, key, ord)

	// Paginate
	return paginateMangas(mangas, opts.Page, opts.PageSize), total, nil
}

// filterByAccessibleLibraries filters mangas to only those in accessible libraries
func filterByAccessibleLibraries(mangas []Manga, accessibleLibraries []string) []Manga {
	if len(accessibleLibraries) == 0 {
		return []Manga{} // No accessible libraries means no manga
	}
	
	// Create a set for O(1) lookup
	librarySet := make(map[string]struct{}, len(accessibleLibraries))
	for _, slug := range accessibleLibraries {
		librarySet[slug] = struct{}{}
	}
	
	filtered := make([]Manga, 0, len(mangas))
	for _, m := range mangas {
		if _, ok := librarySet[m.LibrarySlug]; ok {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// filterByLibrarySlug filters mangas by library slug
func filterByLibrarySlug(mangas []Manga, librarySlug string) []Manga {
	filtered := make([]Manga, 0, len(mangas))
	for _, m := range mangas {
		if m.LibrarySlug == librarySlug {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// filterByAllTags keeps only mangas that have all selected tags
func filterByAllTags(mangas []Manga, selectedTags []string, tagMap map[string][]string) []Manga {
	selectedSet := normalizeTagSet(selectedTags)
	filtered := make([]Manga, 0, len(mangas))
	
	for _, m := range mangas {
		if hasAllTags(tagMap[m.Slug], selectedSet) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// filterByAnyTag keeps only mangas that have at least one of the selected tags
func filterByAnyTag(mangas []Manga, selectedTags []string, tagMap map[string][]string) []Manga {
	selectedSet := normalizeTagSet(selectedTags)
	filtered := make([]Manga, 0, len(mangas))
	
	for _, m := range mangas {
		if hasAnyTag(tagMap[m.Slug], selectedSet) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// normalizeTagSet creates a set of normalized (trimmed, lowercase) tags
func normalizeTagSet(tags []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" {
			set[t] = struct{}{}
		}
	}
	return set
}

// normalizeStringSet lowercases and trims a slice of strings into a set map
func normalizeStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			set[v] = struct{}{}
		}
	}
	return set
}

// hasAllTags checks if tags slice contains all required tags
func hasAllTags(tags []string, required map[string]struct{}) bool {
	if len(required) == 0 {
		return true
	}
	if len(tags) == 0 {
		return false
	}
	
	present := normalizeTagSet(tags)
	for t := range required {
		if _, ok := present[t]; !ok {
			return false
		}
	}
	return true
}

// hasAnyTag checks if tags slice contains at least one tag from the set
func hasAnyTag(tags []string, anySet map[string]struct{}) bool {
	if len(anySet) == 0 {
		return true
	}
	
	for _, t := range tags {
		lt := strings.TrimSpace(strings.ToLower(t))
		if lt == "" {
			continue
		}
		if _, ok := anySet[lt]; ok {
			return true
		}
	}
	return false
}

// paginateMangas applies pagination to manga slice
func paginateMangas(mangas []Manga, page, pageSize int) []Manga {
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start >= len(mangas) {
		return []Manga{}
	}
	
	end := start + pageSize
	if end > len(mangas) {
		end = len(mangas)
	}
	
	return mangas[start:end]
}
