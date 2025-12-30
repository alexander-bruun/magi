package models

import "sort"
import "strings"

// SortOption describes a single allowed sort field and optional alias list.
type SortOption struct {
	Key     string   // canonical key used internally
	Aliases []string // accepted alternative names
}

// GenericSortConfig holds configuration for validating and applying sorts.
type GenericSortConfig struct {
	Allowed      []SortOption
	DefaultKey   string
	DefaultOrder string // "asc" or "desc"
}

// NormalizeSort resolves user supplied sortBy & order into a canonical (key, order).
// Unknown keys fall back to DefaultKey. Unknown order falls back to DefaultOrder.
func (c GenericSortConfig) NormalizeSort(sortBy, order string) (key string, ord string) {
	sb := strings.ToLower(strings.TrimSpace(sortBy))
	ob := strings.ToLower(strings.TrimSpace(order))

	// Determine default order based on sort key
	defaultOrder := c.DefaultOrder
	if sb == "popularity" || sb == "read_count" {
		defaultOrder = "desc"
	}

	if ob != "asc" && ob != "desc" {
		ob = defaultOrder
	}
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

var MediaSortConfig = GenericSortConfig{
	Allowed: []SortOption{
		{Key: "name", Aliases: []string{"title"}},
		{Key: "type"},
		{Key: "year"},
		{Key: "status"},
		{Key: "content_rating", Aliases: []string{"contentrating"}},
		{Key: "created_at", Aliases: []string{"createdat"}},
		{Key: "updated_at", Aliases: []string{"updatedat"}},
		{Key: "read_count", Aliases: []string{"readcount"}},
		{Key: "popularity"},
	},
	DefaultKey:   "name",
	DefaultOrder: "asc",
}

// GetAllowedMediaSortOptions returns sort options, optionally excluding content_rating
// when content rating filtering is active (limit < 3)
func GetAllowedMediaSortOptions() []SortOption {
	cfg, err := GetAppConfig()
	if err != nil {
		// On error, return all options
		return MediaSortConfig.Allowed
	}

	// If content rating limit is less than 3 (not showing all), exclude content_rating from sort
	if cfg.ContentRatingLimit < 3 {
		filtered := make([]SortOption, 0, len(MediaSortConfig.Allowed)-1)
		for _, opt := range MediaSortConfig.Allowed {
			if opt.Key != "content_rating" {
				filtered = append(filtered, opt)
			}
		}
		return filtered
	}

	return MediaSortConfig.Allowed
}

// SortMedias applies the given normalized key & order (use MediaSortConfig.NormalizeSort)
// to the slice in-place.
func SortMedias(media []Media, key, order string) {
	asc := strings.ToLower(order) != "desc"
	switch key {
	case "name":
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) < strings.ToLower(media[j].Name) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) > strings.ToLower(media[j].Name) })
		}
	case "type":
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Type) < strings.ToLower(media[j].Type) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Type) > strings.ToLower(media[j].Type) })
		}
	case "year":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].Year < media[j].Year })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].Year > media[j].Year })
		}
	case "status":
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Status) < strings.ToLower(media[j].Status) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Status) > strings.ToLower(media[j].Status) })
		}
	case "content_rating":
		if asc {
			sort.Slice(media, func(i, j int) bool {
				return strings.ToLower(media[i].ContentRating) < strings.ToLower(media[j].ContentRating)
			})
		} else {
			sort.Slice(media, func(i, j int) bool {
				return strings.ToLower(media[i].ContentRating) > strings.ToLower(media[j].ContentRating)
			})
		}
	case "created_at":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].CreatedAt.Before(media[j].CreatedAt) })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].CreatedAt.After(media[j].CreatedAt) })
		}
	case "updated_at":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].UpdatedAt.Before(media[j].UpdatedAt) })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].UpdatedAt.After(media[j].UpdatedAt) })
		}
	case "read_count":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].ReadCount < media[j].ReadCount })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].ReadCount > media[j].ReadCount })
		}
	case "popularity":
		if asc {
			sort.Slice(media, func(i, j int) bool { return media[i].VoteScore < media[j].VoteScore })
		} else {
			sort.Slice(media, func(i, j int) bool { return media[i].VoteScore > media[j].VoteScore })
		}
	default:
		// default already handled by NormalizeSort -> name
		if asc {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) < strings.ToLower(media[j].Name) })
		} else {
			sort.Slice(media, func(i, j int) bool { return strings.ToLower(media[i].Name) > strings.ToLower(media[j].Name) })
		}
	}
}
