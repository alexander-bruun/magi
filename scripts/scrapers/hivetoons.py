#!/usr/bin/env python3
import json
"""
HiveToons scraper for MAGI.

Downloads manga/manhwa/manhua from hivetoons.org.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

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
    MAX_RETRIES,
    RETRY_DELAY,
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'HiveToons'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[HiveToons]')
ALLOWED_DOMAINS = ['storage.hivetoon.com']
JSON_FILE = os.getenv('json_file', os.path.join(os.path.dirname(__file__), 'hivetoons.json'))
USER_AGENT = os.getenv('user_agent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36')
BASE_URL = 'https://hivetoons.org'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('hivetoons')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from JSON cache.

    Args:
        session: requests.Session object
        page_num: Page number (only page 1 is valid for this source)

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    # hivetoons doesn't have pagination, just load from JSON
    if page_num > 1:
        return [], True
    
    if not os.path.exists(JSON_FILE) or os.path.getsize(JSON_FILE) == 0:
        log("Fetching all series data...")
        response = session.get("https://api.hivetoons.org/api/query?page=1&perPage=99999", timeout=30)
        response.raise_for_status()
        with open(JSON_FILE, 'w') as f:
            f.write(response.text)
    else:
        log("Loading series data from cache...")
    
    with open(JSON_FILE, 'r') as f:
        data = json.load(f)
    
    series_urls = []
    for post in data.get('posts', []):
        if not post.get('isNovel', True):  # Only comics, not novels
            slug = post.get('slug')
            if slug:
                series_urls.append(f"/series/{slug}")
    
    return series_urls, True  # is_last_page = True


def extract_series_title(session, series_url):
    """
    Extract series title from JSON cache.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    series_slug = series_url.replace('/series/', '')
    
    with open(JSON_FILE, 'r') as f:
        data = json.load(f)
    
    for post in data.get('posts', []):
        if post.get('slug') == series_slug:
            return post.get('postTitle')
    
    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter slugs from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: Chapter slugs sorted
    """
    series_slug = series_url.replace('/series/', '')
    full_url = f"https://hivetoons.org/series/{series_slug}"
    
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\n', '')
    
    # Extract chapter slugs
    slugs = re.findall(r'\\"slug\\":\\"chapter-[^"]*\\"', html)
    chapter_slugs = []
    for slug_match in slugs:
        slug = slug_match.replace('\\"slug\\":\\"', '').replace('\\', '').rstrip('"')
        if slug not in chapter_slugs:
            chapter_slugs.append(slug)
    
    # Sort numerically by chapter number
    chapter_slugs.sort(key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
    return chapter_slugs


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs, None if locked, empty list if not found
    """
    full_url = f"https://hivetoons.org{chapter_url}"
    
    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(full_url, timeout=30)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text
            
            # Check if chapter is locked
            if "This chapter is locked" in html:
                return None  # Locked
            
            # Extract image URLs - try the correct JSON pattern first
            images = re.findall(r'"url":"(https://storage\.hivetoon\.com/public/[^"]*)"', html)
            
            # If no images found, extract from src attributes like bash script
            if not images:
                src_matches = re.findall(r'src="([^"]*)"', html)
                images = [url for url in src_matches if urlparse(url).netloc == 'storage.hivetoon.com' and any(url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif'])]
                
                # If still no images, try broader patterns
                if not images:
                    images = re.findall(r'https://storage\.hivetoon\.com/[^\s"]*\.(?:webp|jpg|png|jpeg|avif)', html)
            
            # Filter out UI elements like logos - only keep images from series folders
            filtered_images = [url for url in images if '/upload/series/' in url and 'logo' not in url.lower()]
            
            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(filtered_images))
            
            if len(unique_images) >= 1:
                return unique_images
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(RETRY_DELAY)
    
    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting HiveToons scraper")
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
    page_series, is_last_page = extract_series_urls(session, page)
    all_series_urls.extend(page_series)

    total_series = len(all_series_urls)
    total_chapters = 0

    # Process each series
    for series_url in all_series_urls:
        title = extract_series_title(session, series_url)
        if not title:
            error("No title → skip")
            continue

        clean_title = sanitize_title(title)
        log(f"Title: {clean_title}")
        # Check for duplicate in higher priority providers
        if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
            continue

        series_slug = series_url.replace('/series/', '')

        # Extract chapter slugs
        try:
            chapter_slugs = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        if not chapter_slugs:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for ch_slug in chapter_slugs:
            ch_url = f"/series/{series_slug}/{ch_slug}"
            num_match = re.search(r'chapter-(\d+)', ch_slug)
            if not num_match:
                continue
            num = int(num_match.group(1))

            # Skip if chapter already exists
            if num in existing_chapters:
                continue

            padded = f"{num:02d}"
            chapter_name = format_chapter_name(clean_title, num, 2, DEFAULT_SUFFIX)

            try:
                imgs = extract_image_urls(session, ch_url)
            except Exception as e:
                error(f"Error extracting images for {ch_url}: {e}")
                continue

            if imgs is None:
                log(f"Skipping: Chapter {num} (locked)")
                continue
            elif len(imgs) == 0:
                log(f"Skipping: Chapter {num} (not found)")
                continue
            elif len(imgs) == 1:
                log(f"Skipping: Chapter {num} (only 1 image)")
                continue

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {num} [{len(imgs)} images]")
                continue

            log(f"Downloading: {chapter_name} [{len(imgs)} images]")

            # Create chapter directory within series directory
            chapter_folder = series_directory / chapter_name
            chapter_folder.mkdir(parents=True, exist_ok=True)
            
            downloaded = 0
            total = len(imgs)

            for i, url in enumerate(imgs):
                idx = i + 1
                url = url.replace(' ', '%20')
                ext = get_image_extension(url, 'webp')
                file = chapter_folder / f"{idx:03d}.{ext}"

                try:
                    response = session.get(url, timeout=120)
                    response.raise_for_status()
                    with open(file, 'wb') as f:
                        f.write(response.content)
                    print(f"  [{idx:03d}/{total:03d}] {url} Success", file=sys.stderr, flush=True)
                    downloaded += 1

                    # Convert to WebP if enabled
                    ext = file.suffix.lower()
                    if CONVERT_TO_WEBP and ext != '.webp':
                        convert_to_webp(file)
                except Exception as e:
                    print(f"  [{idx:03d}/{total:03d}] {url} Failed: {e}", file=sys.stderr, flush=True)
                    # Clean up and break
                    shutil.rmtree(chapter_folder)
                    break

            if downloaded != total:
                warn("Incomplete → skipped")
                continue

            if create_cbz(chapter_folder, chapter_name, series_directory):
                shutil.rmtree(chapter_folder)
            else:
                warn(f"CBZ creation failed for Chapter {num}, keeping folder")

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()