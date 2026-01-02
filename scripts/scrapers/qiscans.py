#!/usr/bin/env python3
import json
"""
QiScans scraper for MAGI.

Downloads manga/manhwa/manhua from qiscans.org via their API.
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'QiScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[QiScans]')
ALLOWED_DOMAINS = ['media.qiscans.org']
API_CACHE_FILE = os.path.join(os.path.dirname(__file__), 'qiscans.json')
BASE_URL = 'https://qiscans.org'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('qiscans')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series slugs from the API.

    Args:
        session: requests.Session object
        page_num: Page number (only page 1 is used)

    Returns:
        tuple: (list of series slugs, bool is_last_page)
    """
    # Fetch all series in one go
    if page_num > 1:
        return [], True
    
    if not os.path.exists(API_CACHE_FILE) or os.path.getsize(API_CACHE_FILE) == 0:
        log("Fetching all series data...")
        url = "https://api.qiscans.org/api/query?page=1&perPage=99999"
        response = session.get(url, timeout=30)
        response.raise_for_status()
        data = response.json()
        with open(API_CACHE_FILE, 'w') as f:
            json.dump(data, f)
    else:
        log("Loading series data from cache...")
        with open(API_CACHE_FILE, 'r') as f:
            data = json.load(f)
    
    series_slugs = []
    for post in data.get('posts', []):
        slug = post.get('slug')
        if slug and not slug.startswith('chapter-'):
            series_slugs.append(slug)
    
    return series_slugs, True  # is_last_page = True

def extract_series_title(session, series_slug):
    """
    Extract series title from cached API data.

    Args:
        session: requests.Session object (not used)
        series_slug: Slug of the series

    Returns:
        str: Series title, or empty string if not found
    """
    with open(API_CACHE_FILE, 'r') as f:
        data = json.load(f)
    
    for post in data.get('posts', []):
        if post.get('slug') == series_slug:
            return post.get('postTitle', '')
    
    return ''

def get_series_id(series_slug):
    """
    Get series ID from cached API data.

    Args:
        series_slug: Slug of the series

    Returns:
        int: Series ID, or None if not found
    """
    with open(API_CACHE_FILE, 'r') as f:
        data = json.load(f)
    
    for post in data.get('posts', []):
        if post.get('slug') == series_slug:
            return post.get('id')
    
    return None

# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_slug):
    """
    Extract chapter URLs from the API.

    Args:
        session: requests.Session object
        series_slug: Slug of the series

    Returns:
        list: List of chapter slugs
    """
    series_id = get_series_id(series_slug)
    if not series_id:
        warn(f"Could not find series ID for {series_slug}")
        return []
    
    # Use v2 API to get all chapters
    api_url = f"https://api.qiscans.org/api/v2/posts/{series_id}/chapters?page=1&perPage=9999&sortOrder=asc"
    response = session.get(api_url, timeout=30)
    response.raise_for_status()
    data = response.json()
    
    chapter_slugs = []
    for chapter in data.get('data', []):
        slug = chapter.get('slug')
        if slug and slug not in chapter_slugs:
            # Skip locked/inaccessible chapters
            if chapter.get('isLocked') or not chapter.get('isAccessible', True):
                continue
            chapter_slugs.append(slug)
    
    return chapter_slugs

def extract_image_urls(session, series_slug, chapter_slug):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        series_slug: Slug of the series
        chapter_slug: Slug of the chapter

    Returns:
        list: List of image URLs
    """
    page_url = f"{BASE_URL}/series/{series_slug}/{chapter_slug}"
    response = session.get(page_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check for premium
    if "This premium chapter is waiting to be unlocked" in html:
        return []
    
    # Check for early access
    if "Unlock Early Access chapter by signing in and purchasing" in html:
        return []
    
    # Check for rate limiting
    if "Rate Limited" in html:
        return []
    
    # Extract image URLs
    img_urls = re.findall(r'https://media\.qiscans\.org/file/qiscans/upload/series/[^"]*\.webp', html)
    # Remove /file/qiscans
    img_urls = [url.replace('/file/qiscans', '') for url in img_urls]
    # Exclude thumbnail images (case-insensitive)
    img_urls = [url for url in img_urls if 'thumbnail.webp' not in url.lower()]
    img_urls = list(set(img_urls))
    img_urls.sort()
    
    return img_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Qi Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://qiscans.org", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Get all series slugs
    series_slugs, _ = extract_series_urls(session, 1)
    log(f"Found {len(series_slugs)} series")

    total_series = len(series_slugs)
    total_chapters = 0

    # Process each series
    for series_slug in series_slugs:
        log(f"Processing: {series_slug}")

        title = extract_series_title(session, series_slug)
        if not title:
            warn(f"No title for {series_slug}, skipping...")
            continue

        # Skip novels
        if "[Novel]" in title:
            log(f"Skipping: {title} (novel)")
            continue

        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")
        # Check for duplicate in higher priority providers
        if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
            continue

        # Extract chapter links
        try:
            chapter_slugs = extract_chapter_urls(session, series_slug)
        except Exception as e:
            error(f"Error extracting chapters for {series_slug}: {e}")
            continue

        if not chapter_slugs:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Extract chapter numbers for padding and skipping logic
        chapter_nums = []
        for slug in chapter_slugs:
            match = re.search(r'chapter-(\d+)', slug)
            if match:
                chapter_nums.append(int(match.group(1)))

        if not chapter_nums:
            warn(f"No valid chapter numbers found for {title}, skipping...")
            continue

        max_chapter = max(chapter_nums)
        padding_width = calculate_padding_width(max_chapter)
        log(f"Found {len(chapter_slugs)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Check for existing chapters
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for chapter_slug in chapter_slugs:
            chapter_number_match = re.search(r'chapter-(\d+)', chapter_slug)
            if not chapter_number_match:
                continue
            chapter_number = int(chapter_number_match.group(1))

            # Skip if chapter already exists
            if chapter_number in existing_chapters:
                continue

            chapter_name = format_chapter_name(clean_title, chapter_number, padding_width, DEFAULT_SUFFIX)

            try:
                image_urls = extract_image_urls(session, series_slug, chapter_slug)
            except Exception as e:
                error(f"Error extracting images for chapter {chapter_slug}: {e}")
                continue

            if not image_urls:
                log(f"Skipping: Chapter {chapter_number} (no images)")
                continue

            # Skip if only 1 image
            if len(image_urls) == 1:
                log(f"Skipping: Chapter {chapter_number} (only 1 image)")
                continue

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {chapter_number} [{len(image_urls)} images]")
                continue

            log(f"Downloading: Chapter {chapter_number} [{len(image_urls)} images]")

            # Create chapter directory
            chapter_folder = series_directory / chapter_name
            chapter_folder.mkdir(parents=True, exist_ok=True)

            # Download images
            downloaded_count = 0
            for i, img_url in enumerate(image_urls, 0):
                if not img_url:
                    continue
                # URL encode spaces
                img_url = img_url.replace(' ', '%20')
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