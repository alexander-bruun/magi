#!/bin/bash

# Script to simulate rate limiting by making excessive requests
# This will trigger the rate limiting middleware to block requests

BASE_URL="http://localhost:3000"
REQUESTS=120  # More than the default 100 requests per window
DELAY=0.3     # Delay between requests to fit within 60-second window

echo "Starting rate limiting simulation..."
echo "This will make $REQUESTS requests with ${DELAY}s delay between them"
echo "Default rate limit: 100 requests per 60 seconds"
echo "Press Ctrl+C to stop"

# Function to make a request
make_request() {
    code=$(curl -s -o /dev/null -w "%{http_code}" "$1")
    echo "Request $2: HTTP $code"
}

# Make many requests to trigger rate limiting
for i in $(seq 1 $REQUESTS); do
    make_request "$BASE_URL/api/posters/test.jpg" "$i"
    sleep $DELAY
done

echo "Rate limiting simulation complete."
echo "You should see HTTP 429 (Too Many Requests) responses after the limit is exceeded."