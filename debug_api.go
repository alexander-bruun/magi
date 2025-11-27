package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	// Test the MangaUpdates API directly
	searchURL := "https://api.mangaupdates.com/v1/series/search"

	searchBody := map[string]interface{}{
		"search": "One Punch Man",
		"page":   1,
		"perpage": 10,
	}

	jsonData, err := json.Marshal(searchBody)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	fmt.Printf("Sending request to: %s\n", searchURL)
	fmt.Printf("Request body: %s\n", string(jsonData))

	req, err := http.NewRequest("POST", searchURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Magi/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %s\n", resp.Status)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("Response body: %s\n", string(body))

	// Try to parse as JSON
	var response interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("Failed to parse JSON: %v", err)
	} else {
		fmt.Printf("Parsed response: %+v\n", response)
	}
}