#!/usr/bin/env python3
"""
Omega Scans scraper for MAGI.

Downloads manga/manhwa/manhua from omegascans.org.
"""

# Standard library imports
import asyncio
import json
import os
import re
import shutil
import sys
from pathlib import Path

# Third-party imports
import requests

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    calculate_padding_width,
    convert_to_webp,
    create_cbz,
    error,
    format_chapter_name,
    get_existing_chapters,
    get_image_extension,
    get_session,
    log,
    log_existing_chapters,
    sanitize_title,
    success,
    warn,
    MAX_RETRIES,
    RETRY_DELAY,
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'OmegaScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[OmegaScans]')
ALLOWED_DOMAINS = ['api.omegascans.org', 'media.omegascans.org']
BASE_URL = 'https://omegascans.org'
API_BASE = os.getenv('api', 'https://api.omegascans.org')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series data from API.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, bool is_last_page)
    """
    # Fetch all series in one go
    if page_num > 1:
        return [], True

    url = f"{API_BASE}/query/?page=1&perPage=99999999"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    data = response.json()

    series_data = []
    for series in data.get('data', []):
        series_type = series.get('series_type', '')
        if series_type != 'Novel':  # Skip novels
            series_data.append(series)

    return series_data, True  # is_last_page = True


def extract_series_title(session, series_data):
    """
    Extract series title from series data.

    Args:
        session: requests.Session object (unused, for interface consistency)
        series_data: Series data dictionary from API

    Returns:
        str: Series title
    """
    return series_data.get('title', '')


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_data):
    """
    Extract chapter data from API for a series.

    Args:
        session: requests.Session object
        series_data: Series data dictionary from API

    Returns:
        list: Chapter data dictionaries
    """
    series_id = series_data.get('id')
    url = f"{API_BASE}/chapter/query?page=1&perPage=99999999&series_id={series_id}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    data = response.json()

    chapters = []
    for chapter in data.get('data', []):
        chapter_name_raw = chapter.get('chapter_name', '').strip()

        # Skip seasons and decimal chapters
        if 'season' in chapter_name_raw.lower() or '.' in chapter_name_raw:
            continue

        chapters.append(chapter)

    return chapters


def extract_image_urls(session, chapter_data, series_data):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_data: Chapter data dictionary from API
        series_data: Series data dictionary from API

    Returns:
        list: Image URLs in reading order, empty list if premium/unavailable
    """
    series_slug = series_data.get('series_slug')
    chapter_slug = chapter_data.get('chapter_slug')
    chapter_url = f"{BASE_URL}/series/{series_slug}/{chapter_slug}"

    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Check if premium
    if "This chapter is premium!" in html:
        return []

    # Extract image URLs from src attributes
    api_urls = re.findall(r'src="https://api\.omegascans\.org/uploads/series/[^"]+', html)
    media_urls = re.findall(r'src="https://media\.omegascans\.org/file/[^"]+', html)

    all_urls = api_urls + media_urls
    # Remove src=" prefix
    all_urls = [url.replace('src="', '') for url in all_urls]

    # Remove thumbnail if present
    thumbnail = series_data.get('thumbnail', '')
    if thumbnail:
        all_urls = [url for url in all_urls if url != thumbnail]

    return all_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Omega Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get(BASE_URL, timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Fetch series data
    series_list, _ = extract_series_urls(session, 1)
    log(f"Found {len(series_list)} series")

    total_series = len(series_list)
    total_chapters = 0

    # Process each series
    for series_data in series_list:
        title = extract_series_title(session, series_data)
        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")

        # Fetch chapters
        try:
            chapters = extract_chapter_urls(session, series_data)
        except Exception as e:
            error(f"Error fetching chapters for {title}: {e}")
            continue

        if not chapters:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Determine max chapter number for padding
        chapter_numbers = []
        for chapter in chapters:
            chapter_name_raw = chapter.get('chapter_name', '')
            nums = re.findall(r'\d+', chapter_name_raw)
            if nums:
                chapter_numbers.append(int(nums[0]))

        if not chapter_numbers:
            warn(f"No valid chapter numbers found for {title}, skipping...")
            continue

        max_chapter = max(chapter_numbers)
        padding_width = calculate_padding_width(max_chapter)

        log(f"Found {len(chapters)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for chapter in chapters:
            chapter_name_raw = chapter.get('chapter_name', '').strip()
            nums = re.findall(r'\d+', chapter_name_raw)
            chapter_number = int(nums[0]) if nums else 0

            # Skip if chapter already exists
            if chapter_number in existing_chapters:
                continue

            total_chapters += 1

            chapter_name = format_chapter_name(clean_title, chapter_number, padding_width, DEFAULT_SUFFIX)

            try:
                image_urls = extract_image_urls(session, chapter, series_data)
            except Exception as e:
                error(f"Error extracting images for chapter {chapter_name_raw}: {e}")
                continue

            if not image_urls:
                log(f"Skipping: Chapter {chapter_number} (no images)")
                continue

            if DRY_RUN:
                log(f"Chapter {chapter_number} [{len(image_urls)} images]")
                continue

            log(f"Downloading: Chapter {chapter_number} [{len(image_urls)} images]")

            # Create chapter directory
            chapter_folder = series_directory / chapter_name
            chapter_folder.mkdir(parents=True, exist_ok=True)

            # Download images
            downloaded_count = 0
            for i, img_url in enumerate(image_urls, 1):
                if not img_url:
                    continue
                ext = get_image_extension(img_url, 'jpg')
                filename = chapter_folder / f"{i:03d}.{ext}"
                try:
                    response = session.get(img_url, timeout=30)
                    response.raise_for_status()
                    with open(filename, 'wb') as f:
                        f.write(response.content)
                    print(f"  [{i:03d}/{len(image_urls):03d}] {img_url} Success", file=sys.stderr, flush=True)
                    downloaded_count += 1
                    if CONVERT_TO_WEBP and ext != 'webp':
                        convert_to_webp(filename)
                except Exception as e:
                    print(f"  [{i:03d}/{len(image_urls):03d}] {img_url} Failed: {e}", file=sys.stderr, flush=True)

            # Only create CBZ if more than 1 image was downloaded
            if downloaded_count > 1:
                if create_cbz(chapter_folder, chapter_name):
                    # Remove temp folder
                    shutil.rmtree(chapter_folder)
                else:
                    warn(f"CBZ creation failed for Chapter {chapter_number}, keeping folder")
            else:
                log(f"Skipping CBZ creation for Chapter {chapter_number} - only {downloaded_count} image(s) downloaded")
                # Remove temp folder
                shutil.rmtree(chapter_folder)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()