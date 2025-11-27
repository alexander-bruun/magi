package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	// Test getting a specific series
	seriesURL := "https://api.mangaupdates.com/v1/series/30535847369"

	resp, err := http.Get(seriesURL)
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
}