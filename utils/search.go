package utils

// BigramSearch performs a bigram search algorithm
func BigramSearch(keyword string, items []string) []string {
	var results []string
	for _, item := range items {
		score := CompareStrings(keyword, item)
		// For simplicity, consider a threshold of 0.3 for similarity
		if score > 0.3 {
			results = append(results, item)
		}
	}
	return results
}

// compareStrings compares two strings using a bigram-based similarity algorithm
func CompareStrings(str1, str2 string) float64 {
	if str1 == str2 {
		return 1
	}

	len1 := len(str1)
	len2 := len(str2)
	if len1 < 2 || len2 < 2 {
		return 0
	}

	bigramCounts := make(map[string]int)
	commonBigramsCount := 0
	totalBigrams := 0

	// Process the first string
	for i := 0; i < len1-1; i++ {
		bigram := str1[i : i+2]
		bigramCounts[bigram]++
	}

	// Process the second string and calculate common bigrams
	for i := 0; i < len2-1; i++ {
		bigram := str2[i : i+2]
		if bigramCounts[bigram] > 0 {
			commonBigramsCount++
			bigramCounts[bigram]--
		}
		totalBigrams++
	}

	// Include remaining bigrams from the first string
	totalBigrams += len1 - 1

	return (2.0 * float64(commonBigramsCount)) / float64(totalBigrams)
}
