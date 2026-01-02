#!/bin/bash

# Setup virtual environment and start interactive scraper

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Activate the virtual environment (assuming it's in the parent directory of scripts)
source "$SCRIPT_DIR/../.venv/bin/activate"

# Change to the scrapers directory
cd "$SCRIPT_DIR/scrapers"

# Run the interactive scraper
python interactive_scraper.py "$@"