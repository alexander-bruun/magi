package main

import (
	"fmt"
	"log"
	"github.com/alexander-bruun/magi/metadata"
)

func main() {
	// Test the MangaUpdates provider
	provider, err := metadata.GetProvider("mangaupdates", "")
	if err != nil {
		log.Fatalf("Failed to create MangaUpdates provider: %v", err)
	}

	fmt.Printf("Testing MangaUpdates provider: %s\n", provider.Name())

	// Test search for "One Punch Man"
	fmt.Println("\n=== Searching for 'One Punch Man' ===")
	results, err := provider.Search("One Punch Man")
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	fmt.Printf("Found %d results:\n", len(results))
	for i, result := range results {
		fmt.Printf("%d. %s (ID: %s, Year: %d, Score: %.2f)\n",
			i+1, result.Title, result.ID, result.Year, result.SimilarityScore)
		if result.Description != "" {
			// Truncate description for readability
			desc := result.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			fmt.Printf("   Description: %s\n", desc)
		}
		if len(result.Tags) > 0 {
			fmt.Printf("   Tags: %v\n", result.Tags)
		}
		fmt.Println()
	}

	// Test getting metadata for the best match
	if len(results) > 0 {
		fmt.Println("=== Getting metadata for best match ===")
		bestResult := results[0]
		for _, result := range results {
			if result.SimilarityScore > bestResult.SimilarityScore {
				bestResult = result
			}
		}

		fmt.Printf("Fetching metadata for: %s (ID: %s)\n", bestResult.Title, bestResult.ID)
		metadata, err := provider.GetMetadata(bestResult.ID)
		if err != nil {
			log.Fatalf("Failed to get metadata: %v", err)
		}

		fmt.Printf("Title: %s\n", metadata.Title)
		fmt.Printf("Year: %d\n", metadata.Year)
		fmt.Printf("Status: %s\n", metadata.Status)
		fmt.Printf("Type: %s\n", metadata.Type)
		fmt.Printf("Content Rating: %s\n", metadata.ContentRating)
		if metadata.Author != "" {
			fmt.Printf("Author: %s\n", metadata.Author)
		}
		if len(metadata.Tags) > 0 {
			fmt.Printf("Tags: %v\n", metadata.Tags)
		}
		if len(metadata.AlternativeTitles) > 0 {
			fmt.Printf("Alternative Titles: %v\n", metadata.AlternativeTitles)
		}
		if metadata.CoverArtURL != "" {
			fmt.Printf("Cover URL: %s\n", metadata.CoverArtURL)
		}
		if metadata.Description != "" {
			desc := metadata.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			fmt.Printf("Description: %s\n", desc)
		}
	}

	// Test FindBestMatch
	fmt.Println("\n=== Testing FindBestMatch ===")
	bestMatch, err := provider.FindBestMatch("One Punch Man")
	if err != nil {
		log.Fatalf("FindBestMatch failed: %v", err)
	}

	fmt.Printf("Best match found: %s (Year: %d, Status: %s)\n",
		bestMatch.Title, bestMatch.Year, bestMatch.Status)
}