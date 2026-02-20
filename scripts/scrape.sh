#!/bin/bash

# Setup virtual environment and start interactive scraper

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to the scrapers directory
cd "$SCRIPT_DIR/scrapers"

if [ "$1" = "all" ]; then
    # Run all scrapers in sequence
    for scraper in $(ls *.py | grep -v scraper_utils.py | grep -v comix.py); do
        echo "Running $scraper..."
        python "$scraper"
    done
else
    # Run the specified scraper
    python "$@.py"
fi