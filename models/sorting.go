package models

import "sort"
import "strings"

// SortOption describes a single allowed sort field and optional alias list.
type SortOption struct {
	Key    string   // canonical key used internally
	Aliases []string // accepted alternative names
}

// GenericSortConfig holds configuration for validating and applying sorts.
type GenericSortConfig struct {
	Allowed []SortOption
	DefaultKey string
	DefaultOrder string // "asc" or "desc"
}

// NormalizeSort resolves user supplied sortBy & order into a canonical (key, order).
// Unknown keys fall back to DefaultKey. Unknown order falls back to DefaultOrder.
func (c GenericSortConfig) NormalizeSort(sortBy, order string) (key string, ord string) {
	sb := strings.ToLower(strings.TrimSpace(sortBy))
	ob := strings.ToLower(strings.TrimSpace(order))
	if ob != "asc" && ob != "desc" { ob = c.DefaultOrder }
	key = c.DefaultKey
	for _, opt := range c.Allowed {
		if sb == opt.Key {
			key = opt.Key
			break
		}
		for _, a := range opt.Aliases {
			if sb == strings.ToLower(a) {
				key = opt.Key
				break
			}
		}
	}
	return key, ob
}

var MangaSortConfig = GenericSortConfig{
	Allowed: []SortOption{
		{Key: "name", Aliases: []string{"title"}},
		{Key: "type"},
		{Key: "year"},
		{Key: "status"},
		{Key: "content_rating", Aliases: []string{"contentrating"}},
		{Key: "created_at", Aliases: []string{"createdat"}},
		{Key: "updated_at", Aliases: []string{"updatedat"}},
	},
	DefaultKey: "name",
	DefaultOrder: "asc",
}

// GetAllowedMangaSortOptions returns sort options, optionally excluding content_rating
// when content rating filtering is active (limit < 3)
func GetAllowedMangaSortOptions() []SortOption {
	cfg, err := GetAppConfig()
	if err != nil {
		// On error, return all options
		return MangaSortConfig.Allowed
	}
	
	// If content rating limit is less than 3 (not showing all), exclude content_rating from sort
	if cfg.ContentRatingLimit < 3 {
		filtered := make([]SortOption, 0, len(MangaSortConfig.Allowed)-1)
		for _, opt := range MangaSortConfig.Allowed {
			if opt.Key != "content_rating" {
				filtered = append(filtered, opt)
			}
		}
		return filtered
	}
	
	return MangaSortConfig.Allowed
}

// SortMangas applies the given normalized key & order (use MangaSortConfig.NormalizeSort)
// to the slice in-place.
func SortMangas(mangas []Manga, key, order string) {
	asc := strings.ToLower(order) != "desc"
	switch key {
	case "name":
		if asc {
			sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Name) < strings.ToLower(mangas[j].Name) })
		} else {
			sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Name) > strings.ToLower(mangas[j].Name) })
		}
	case "type":
		if asc { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Type) < strings.ToLower(mangas[j].Type) }) } else { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Type) > strings.ToLower(mangas[j].Type) }) }
	case "year":
		if asc { sort.Slice(mangas, func(i, j int) bool { return mangas[i].Year < mangas[j].Year }) } else { sort.Slice(mangas, func(i, j int) bool { return mangas[i].Year > mangas[j].Year }) }
	case "status":
		if asc { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Status) < strings.ToLower(mangas[j].Status) }) } else { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Status) > strings.ToLower(mangas[j].Status) }) }
	case "content_rating":
		if asc { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].ContentRating) < strings.ToLower(mangas[j].ContentRating) }) } else { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].ContentRating) > strings.ToLower(mangas[j].ContentRating) }) }
	case "created_at":
		if asc { sort.Slice(mangas, func(i, j int) bool { return mangas[i].CreatedAt.Before(mangas[j].CreatedAt) }) } else { sort.Slice(mangas, func(i, j int) bool { return mangas[i].CreatedAt.After(mangas[j].CreatedAt) }) }
	case "updated_at":
		if asc { sort.Slice(mangas, func(i, j int) bool { return mangas[i].UpdatedAt.Before(mangas[j].UpdatedAt) }) } else { sort.Slice(mangas, func(i, j int) bool { return mangas[i].UpdatedAt.After(mangas[j].UpdatedAt) }) }
	default:
		// default already handled by NormalizeSort -> name
		if asc { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Name) < strings.ToLower(mangas[j].Name) }) } else { sort.Slice(mangas, func(i, j int) bool { return strings.ToLower(mangas[i].Name) > strings.ToLower(mangas[j].Name) }) }
	}
}