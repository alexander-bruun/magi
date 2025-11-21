#!/bin/bash

# Script to simulate bot behavior by making excessive requests to manga series and chapters
# This will trigger the bot detection middleware to ban the IP

BASE_URL="http://localhost:3000"
MANGA_SLUG="what-a-bountiful-harvest-demon-lord"  # Use a real manga slug
CHAPTER_SLUG="what-a-bountiful-harvest-demon-lord-ch-01"  # Use a real chapter slug

echo "Starting bot simulation..."
echo "This will make excessive requests to trigger IP banning"
echo "Press Ctrl+C to stop"

# Function to make a request
make_request() {
    code=$(curl -s -o /dev/null -w "%{http_code}" "$1")
    echo "Request to $1: HTTP $code"
}

# Simulate series accesses (threshold: 5 in 60 seconds)
echo "Making series accesses..."
for i in {1..8}; do
    echo "Series request $i"
    make_request "$BASE_URL/mangas/$MANGA_SLUG"
done

# Simulate chapter accesses (threshold: 10 in 60 seconds)
echo "Making chapter accesses..."
for i in {1..15}; do
    echo "Chapter request $i"
    make_request "$BASE_URL/mangas/$MANGA_SLUG/chapters/$CHAPTER_SLUG"
done

echo "Simulation complete. Check if your IP was banned at /admin/banned-ips"