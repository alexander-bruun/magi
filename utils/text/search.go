package text

import (
	"strings"
)

// BigramSearch finds items that are similar to the given keyword based on bigram similarity.
func BigramSearch(keyword string, items []string) []string {
	// Normalize keyword for case-insensitive search
	keywordLower := strings.ToLower(keyword)

	var results []string
	for _, item := range items {
		if score := CompareStrings(keywordLower, strings.ToLower(item)); score > 0.3 {
			results = append(results, item)
		}
	}
	return results
}

// CompareStrings computes a similarity score between two strings using bigrams.
func CompareStrings(str1, str2 string) float64 {
	if str1 == str2 {
		return 1.0
	}

	if len(str1) < 2 || len(str2) < 2 {
		return 0.0
	}

	bigramCounts := buildBigramCounts(str1)
	commonBigramsCount, totalBigrams := countCommonBigrams(str2, bigramCounts)

	// Include remaining bigrams from the first string
	totalBigrams += len(str1) - 1

	// Calculate base similarity score
	baseScore := (2.0 * float64(commonBigramsCount)) / float64(totalBigrams)

	// Add a boost if one string is a substring of the other
	boost := 0.0
	if strings.Contains(str1, str2) || strings.Contains(str2, str1) {
		boost = 0.2
	}

	return baseScore + boost
}

// buildBigramCounts creates a map of bigram counts for the given string.
func buildBigramCounts(str string) map[string]int {
	bigramCounts := make(map[string]int)
	for i := 0; i < len(str)-1; i++ {
		bigram := str[i : i+2]
		bigramCounts[bigram]++
	}
	return bigramCounts
}

// countCommonBigrams counts the number of common bigrams between two strings.
func countCommonBigrams(str string, bigramCounts map[string]int) (commonBigramsCount, totalBigrams int) {
	for i := 0; i < len(str)-1; i++ {
		bigram := str[i : i+2]
		if count := bigramCounts[bigram]; count > 0 {
			commonBigramsCount++
			bigramCounts[bigram]--
		}
		totalBigrams++
	}
	return
}
