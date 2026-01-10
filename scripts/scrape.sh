#!/bin/bash

# Setup virtual environment and start interactive scraper

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to the scrapers directory
cd "$SCRIPT_DIR/scrapers"

# Run the interactive scraper
python "$@.py"