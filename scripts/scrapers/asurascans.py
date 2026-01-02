#!/usr/bin/env python3
"""
Asura Scans scraper for MAGI.

Downloads manga/manhwa/manhua from asuracomic.net.
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
    MAX_RETRIES,
    RETRY_DELAY,
    calculate_padding_width,
    convert_to_webp,
    create_cbz,
    check_duplicate_series,
    get_priority_config,
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
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'AsuraScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[AsuraScans]')
ALLOWED_DOMAINS = ['gg.asuracomic.net']
BASE_URL = 'https://asuracomic.net'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('asurascans')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    url = f"{BASE_URL}/series?page={page_num}"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for disabled "Next" button
    is_last_page = 'pointer-events: none' in html and 'Next' in html
    
    # Match series URLs with the 8-character hex suffix pattern (e.g., series/nano-machine-be19545a)
    series_urls = re.findall(r'href="series/[a-z0-9-]+-[a-f0-9]{8}"', html)
    # Remove href=" and add leading /
    series_urls = [url.replace('href="', '/').rstrip('"') for url in series_urls]
    return sorted(set(series_urls)), is_last_page


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL path of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract title from <title> tag and remove " - Asura Scans" suffix
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' - Asura Scans', '').strip()
                return title
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(f"Failed to extract title (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract title after {MAX_RETRIES} attempts: {e}")
                return None

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL path of the series

    Returns:
        list: Chapter URLs sorted by chapter number
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    series_slug = series_url.split('/series/')[-1]
    # Extract chapter links like series_slug/chapter/123
    chapter_patterns = re.findall(rf'{re.escape(series_slug)}/chapter/\d+', html)
    # Convert to full URLs
    chapter_urls = [f'/series/{pattern}' for pattern in chapter_patterns]
    # Sort by chapter number
    chapter_urls.sort(key=lambda x: int(re.search(r'/chapter/(\d+)', x).group(1)))
    return list(dict.fromkeys(chapter_urls))  # unique


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL path of the chapter

    Returns:
        list: Image URLs in reading order, empty list if unavailable
    """
    full_url = f"{BASE_URL}{chapter_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract image URLs from JSON data embedded in the page
            # The page contains escaped JSON with "order" and "url" fields
            # Try general pattern first
            pattern = r'\\"order\\":\d+,\\"url\\":\\"https://[^"\\]+\.(?:jpg|webp)\\"'
            matches = re.findall(pattern, html)
            
            if matches:
                # Extract URLs and sort by order
                urls_with_order = []
                for match in matches:
                    # Extract order and url
                    order_match = re.search(r'\\"order\\":(\d+)', match)
                    url_match = re.search(r'\\"url\\":\\"([^"\\]+)\\"', match)
                    if order_match and url_match:
                        order = int(order_match.group(1))
                        url = url_match.group(1)
                        urls_with_order.append((order, url))

                # Sort by order and extract URLs
                urls_with_order.sort(key=lambda x: x[0])
                image_urls = [url for _, url in urls_with_order]
                # Remove duplicates while preserving order
                seen = set()
                image_urls = [x for x in image_urls if not (x in seen or seen.add(x))]
                return image_urls
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(f"Failed to extract images (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract images after {MAX_RETRIES} attempts: {e}")
                return []

    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Asura Scans scraper")
    log("Mode: Full Downloader")

    # Create session
    session = get_session()

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Collect all series URLs
    all_series_urls = []
    page = 1
    while True:
        try:
            page_series, is_last_page = extract_series_urls(session, page)
            if not page_series:
                log(f"No series found on page {page}, stopping.")
                break
            all_series_urls.extend(page_series)
            log(f"Found {len(page_series)} series on page {page}")
            if is_last_page:
                log(f"Reached last page (page {page}).")
                break
            page += 1
        except Exception as e:
            error(f"Error fetching page {page}: {e}")
            break

    log(f"Found {len(all_series_urls)} series")

    total_series = len(all_series_urls)
    total_chapters = 0

    # Process each series
    for series_url in all_series_urls:
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
            chapter_urls = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        if not chapter_urls:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Determine padding width
        max_chapter = max(int(re.search(r'/chapter/(\d+)', url).group(1)) for url in chapter_urls)
        padding_width = calculate_padding_width(max_chapter)
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        consecutive_skips = 0
        for chapter_url in chapter_urls:
            chapter_num_match = re.search(r'/chapter/(\d+)', chapter_url)
            if not chapter_num_match:
                continue
            chapter_num = int(chapter_num_match.group(1))

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
                consecutive_skips += 1
                if consecutive_skips >= 3:  # Stop after 3 consecutive non-existent chapters
                    log("Stopping due to 3 consecutive non-existent chapters")
                    break
                continue

            consecutive_skips = 0  # Reset on successful find

            if DRY_RUN:
                log(f"Chapter {chapter_num} [{len(image_urls)} images]")
                continue

            log(f"Downloading: {chapter_name} [{len(image_urls)} images]")

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
                    warn(f"CBZ creation failed for Chapter {chapter_num}, keeping folder")
            else:
                log(f"Skipping CBZ creation for Chapter {chapter_num} - only {downloaded_count} image(s) downloaded")
                # Remove temp folder
                shutil.rmtree(chapter_folder)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()