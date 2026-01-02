#!/usr/bin/env python3
"""
Demonic Scans scraper for MAGI.

Downloads manga/manhwa/manhua from demonicscans.org.
"""

# Standard library imports
import asyncio
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'DemonicScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[DemonicScans]')
ALLOWED_DOMAINS = ['demoniclibs.com', 'mangareadon.org', 'readermc.org', 'mangafirst.org']
BASE_URL = 'https://demonicscans.org'


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from the translations list page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    url = f"{BASE_URL}/translationlist.php?page={page_num}"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for disabled "Next" button
    is_last_page = 'pointer-events: none' in html and 'Next' in html
    
    # Extract series URLs
    series_urls = re.findall(r'href="(/manga/[^"]+)"', html)
    return sorted(set(series_urls)), is_last_page


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        title = title_match.group(1).replace(' - Demonic Scans', '').strip()
        # Replace underscores with spaces (they may come from the HTML)
        title = title.replace('_', ' ')
        return title
    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, manga_url):
    """
    Extract chapter URLs for a given manga.

    Args:
        session: requests.Session object
        manga_url: Relative URL of the manga

    Returns:
        list: Chapter URLs sorted by chapter number
    """
    full_url = f"{BASE_URL}{manga_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\0', '')
    
    chapter_urls = re.findall(r'href="(/chaptered\.php\?manga=[^&]+&chapter=[^"]+)"', html)
    return sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter=(\d+)', x).group(1)))


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs for a given chapter.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs in reading order
    """
    full_url = f"{BASE_URL}{chapter_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\0', '')
    
    image_urls = re.findall(r'https?://[^"]*\.(?:jpg|jpeg|png|webp|web)', html)
    filtered_urls = [url for url in image_urls if any(domain in url for domain in ALLOWED_DOMAINS) and 'thumbnails' not in url]
    return list(dict.fromkeys(filtered_urls))  # unique


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Demonic Scans scraper")
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
        max_chapter = max(int(re.search(r'chapter=(\d+)', url).group(1)) for url in chapter_urls)
        padding_width = calculate_padding_width(max_chapter)
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for chapter_url in chapter_urls:
            chapter_num = int(re.search(r'chapter=(\d+)', chapter_url).group(1))

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
                if not img_url or not any(domain in img_url for domain in ALLOWED_DOMAINS):
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