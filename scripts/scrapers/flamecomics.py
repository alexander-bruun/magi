#!/usr/bin/env python3
"""
Flame Comics scraper for MAGI.

Downloads manga/manhwa/manhua from flamecomics.xyz.
"""

# Standard library imports
import os
import re
import shutil
import sys
import time
from pathlib import Path

# Local imports
from scraper_utils import (
    calculate_padding_width,
    convert_to_webp,
    create_cbz,
    check_duplicate_series,
    get_priority_config,
    error,
    format_chapter_name,
    get_default_headers,
    get_existing_chapters,
    get_image_extension,
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'FlameComics'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[FlameComics]')
ALLOWED_DOMAINS = ['cdn.flamecomics.xyz']
BASE_URL = 'https://flamecomics.xyz'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('flamecomics')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from browse page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    # Flame Comics doesn't have pagination, just one browse page
    if page_num > 1:
        return [], True

    url = f"{BASE_URL}/browse"
    log("Fetching series from browse page...")

    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract series URLs (numeric IDs)
    series_urls = re.findall(r'href="/series/(\d+)"', html)
    full_urls = [f"{BASE_URL}/series/{sid}" for sid in series_urls]
    return sorted(set(full_urls)), True  # is_last_page = True since no pagination


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text

            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' - Flame Comics', '').strip()
                if title:
                    return title
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(f"Failed to extract title (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
                time.sleep(RETRY_DELAY)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        list: List of tuples (chapter_num, chapter_url) sorted by chapter number
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract __NEXT_DATA__ JSON
    json_match = re.search(r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL)
    if not json_match:
        error(f"Could not find __NEXT_DATA__ in {series_url}")
        return []

    try:
        data = json.loads(json_match.group(1))
        chapters = data.get('props', {}).get('pageProps', {}).get('chapters', [])

        # Sort by chapter number
        chapters.sort(key=lambda x: float(x.get('chapter', 0)))

        chapter_data = []
        for chapter in chapters:
            series_id = chapter.get('series_id')
            token = chapter.get('token')
            chapter_num = float(chapter.get('chapter', 0))
            if series_id and token:
                url = f"{BASE_URL}/series/{series_id}/{token}"
                chapter_data.append((chapter_num, url))

        return chapter_data
    except (json.JSONDecodeError, KeyError) as e:
        error(f"Error parsing JSON from {series_url}: {e}")
        return []


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs in reading order, empty list if unavailable
    """
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(chapter_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract __NEXT_DATA__ JSON
            json_match = re.search(r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL)
            if not json_match:
                if attempt < MAX_RETRIES:
                    time.sleep(RETRY_DELAY)
                continue

            data = json.loads(json_match.group(1))
            chapter_data = data.get('props', {}).get('pageProps', {}).get('chapter', {})
            images = chapter_data.get('images', {})
            series_id = chapter_data.get('series_id')
            token = chapter_data.get('token')

            if not series_id or not token:
                if attempt < MAX_RETRIES:
                    time.sleep(RETRY_DELAY)
                continue

            urls = []
            for key, img_data in images.items():
                name = img_data.get('name', '')
                if 'commission' not in name:
                    url = f"https://cdn.flamecomics.xyz/uploads/images/series/{series_id}/{token}/{name}"
                    urls.append(url)

            if urls:
                return urls
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(f"Failed to extract images (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
                time.sleep(RETRY_DELAY)

    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    try:
        log("Starting Flame Comics scraper")
        log("Mode: Full Downloader")

        # Health check
        log(f"Performing health check on {BASE_URL}...")
        try:
            session = requests.Session()
            session.headers.update(get_default_headers())

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

        # Collect all series URLs
        all_series_urls = []
        page = 1
        page_series, is_last_page = extract_series_urls(session, page)
        all_series_urls.extend(page_series)

        log(f"Found {len(all_series_urls)} series")

        total_series = len(all_series_urls)
        total_chapters = 0

        # Process each series
        for series_url in all_series_urls:
            if not series_url:
                continue

            log(f"Processing: {series_url}")

            title = extract_series_title(session, series_url)
            if not title:
                error(f"Could not extract title for {series_url}, skipping...")
                continue

            clean_title = sanitize_title(title)

            log(f"Title: {clean_title}")
            # Check for duplicate in higher priority providers
            if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
                continue

            # Extract chapter URLs
            try:
                chapter_data = extract_chapter_urls(session, series_url)
            except Exception as e:
                error(f"Error extracting chapters for {series_url}: {e}")
                continue

            if not chapter_data:
                warn(f"No chapters found for {title}, skipping...")
                continue

            # Create series directory (only after confirming chapters exist)
            series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
            series_directory.mkdir(parents=True, exist_ok=True)

            # For Flame Comics, get actual chapter numbers
            chapter_numbers = [chapter_num for chapter_num, _ in chapter_data]
            max_chapter_number = max(chapter_numbers) if chapter_numbers else 0
            
            # Determine padding width based on the integer part of chapter numbers
            integer_parts = [int(chapter_num) for chapter_num in chapter_numbers]
            max_integer = max(integer_parts) if integer_parts else 0
            padding_width = calculate_padding_width(max_integer)
            log(f"Found {len(chapter_data)} chapters (max: {max_chapter_number}, padding: {padding_width})")

            # Scan existing CBZ files to determine which chapters are already downloaded
            existing_chapters = get_existing_chapters(series_directory)
            log_existing_chapters(existing_chapters)

            for chapter_num, chapter_url in chapter_data:
                # Skip if chapter already exists
                if chapter_num in existing_chapters:
                    continue

                total_chapters += 1

                chapter_name = format_chapter_name(clean_title, chapter_num, padding_width, DEFAULT_SUFFIX)

                try:
                    image_urls = extract_image_urls(session, chapter_url)
                except Exception as e:
                    error(f"Error extracting images for {chapter_url}: {e}")
                    continue

                if not image_urls:
                    log(f"Skipping: Chapter {chapter_num} (no images)")
                    continue

                if DRY_RUN:
                    log(f"Chapter {chapter_num} [{len(image_urls)} images]")
                    continue

                log(f"Downloading: Chapter {chapter_num} [{len(image_urls)} images]")

                # Create chapter directory
                chapter_folder = series_directory / chapter_name
                chapter_folder.mkdir(parents=True, exist_ok=True)

                # Download images
                downloaded_count = 0
                for i, img_url in enumerate(image_urls, 1):
                    if not img_url:
                        continue
                    ext = get_image_extension(img_url, 'webp')
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
                        warn(f"CBZ creation failed for Chapter {chapter_num}, keeping folder")
                else:
                    log(f"Skipping CBZ creation for Chapter {chapter_num} - only {downloaded_count} image(s) downloaded")
                    # Remove temp folder
                    shutil.rmtree(chapter_folder)

        log(f"Total series processed: {total_series}")
        log(f"Total chapters downloaded: {total_chapters}")
        success(f"Completed! Output: {FOLDER}")
    except Exception as e:
        error(f"Unexpected error in main(): {e}")
        import traceback
        traceback.print_exc()


if __name__ == '__main__':
    main()