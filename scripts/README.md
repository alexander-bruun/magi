# Scripts

This directory contains utility scripts for the Magi project.

## Contents

- `build-release.sh`: Builds the Magi application for multiple platforms (Windows, Linux, macOS on amd64 and arm64) using Zig and creates SHA256 checksums.
- `coverage.sh`: Runs Go tests with coverage reporting and displays coverage percentages by folder.
- `generate-dummy-data.sh`: Generates dummy manga folder structures with chapter files for testing purposes.
- `scrapers/`: Directory containing Python scripts for scraping manga metadata from various websites (e.g., AsuraScans, DemonicScans, FlameComics, etc.), along with configuration files and utilities.
- `setup_scraper.sh`: Sets up the Python virtual environment and launches the interactive scraper.
- `simulate_bot.sh`: Simulates bot behavior by making excessive requests to trigger IP banning in the bot detection middleware.
- `simulate_rate_limit.sh`: Simulates rate limiting by making many requests to test the rate limiting middleware.
- `update_deps.sh`: Updates all Go dependencies to their latest versions and verifies the build.
