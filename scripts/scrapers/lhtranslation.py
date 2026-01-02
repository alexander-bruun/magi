#!/usr/bin/env python3
import json
"""
LHTranslation scraper for MAGI.

Downloads manga/manhwa/manhua from lhtranslation.net.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import sys
import time
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
    MAX_RETRIES,
    RETRY_DELAY,
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'LHTranslation'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[LHTranslation]')
ALLOWED_DOMAINS = ['lhtranslation.net']
BASE_URL = 'https://lhtranslation.net'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('lhtranslation')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page with load more.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    if page_num == 1:
        # First page: direct fetch
        url = f"{BASE_URL}/manga/"
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text
    else:
        # Subsequent pages: load more via AJAX
        ajax_url = f"{BASE_URL}/wp-admin/admin-ajax.php"
        data = {
            'action': 'madara_load_more',
            'page': page_num,
            'template': 'madara-core/content/content-archive',
            'vars[paged]': page_num,
            'vars[orderby]': 'meta_value_num',
            'vars[template]': 'archive',
            'vars[sidebar]': 'right',
            'vars[post_type]': 'wp-manga',
            'vars[post_status]': 'publish',
            'vars[meta_key]': '_latest_update',
            'vars[order]': 'desc',
            'vars[meta_query][relation]': 'AND',
            'vars[manga_archives_item_layout]': 'default'
        }
        response = session.post(ajax_url, data=data, timeout=30)
        response.raise_for_status()
        html = response.text
    
    # Extract series URLs
    series_urls = re.findall(r'href="https://lhtranslation\.net/manga/[^"]*/"', html)
    # Remove href=" and " and filter out chapter and feed URLs
    series_urls = [url.replace('href="', '').rstrip('"') for url in series_urls 
                   if '/chapter-' not in url and '/feed/' not in url]
    
    is_last_page = len(series_urls) == 0
    return sorted(set(series_urls)), is_last_page


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if not found
    """
    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' &#8211; LHTranslation', '').strip()
                if title:
                    return title
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(f"Failed to extract title (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
                time.sleep(RETRY_DELAY)
    
    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page via AJAX.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        list: Chapter URLs sorted
    """
    ajax_url = f"{series_url}ajax/chapters/"
    response = session.post(ajax_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter URLs
    chapter_urls = re.findall(r'href="https://lhtranslation\.net/manga/[^"]*chapter-[^"]*/"', html)
    chapter_urls = [url.replace('href="', '').rstrip('"') for url in chapter_urls]
    
    # Sort numerically by chapter number
    unique_urls = list(set(chapter_urls))
    unique_urls.sort(key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
    return unique_urls


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\n', ' ')
    
    # Extract image URLs from data-src attributes
    image_urls = re.findall(r'data-src="\s*(https://lhtranslation\.net/wp-content/uploads/WP-manga/data/[^"]*\.(?:jpg|jpeg|png|webp))', html)
    # Remove duplicates while preserving order
    return list(dict.fromkeys(image_urls))


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting LHTranslation scraper")
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
        series_directory = Path(FOLDER) / clean_title
        series_directory.mkdir(parents=True, exist_ok=True)

        # Extract chapter numbers for padding and skipping logic
        chapter_nums = []
        for url in chapter_urls:
            match = re.search(r'chapter-([^/]+)', url)
            if match:
                try:
                    chapter_nums.append(int(match.group(1)))
                except ValueError:
                    continue
        
        if not chapter_nums:
            warn(f"No valid chapter numbers found for {title}, skipping...")
            continue

        max_chapter = max(chapter_nums)
        padding_width = calculate_padding_width(max_chapter)
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for chapter_url in chapter_urls:
            chapter_num_match = re.search(r'chapter-([^/]+)', chapter_url)
            if not chapter_num_match:
                continue
            try:
                chapter_num = int(chapter_num_match.group(1))
            except ValueError:
                continue

            # Skip if chapter already exists
            if chapter_num in existing_chapters:
                continue

            chapter_name = format_chapter_name(clean_title, chapter_num, padding_width, DEFAULT_SUFFIX)

            try:
                image_urls = extract_image_urls(session, chapter_url)
            except Exception as e:
                error(f"Error extracting images for {chapter_url}: {e}")
                continue

            if not image_urls:
                log(f"Skipping: Chapter {chapter_num} (no images)")
                continue

            total_chapters += 1

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
                    warn(f"CBZ creation failed for {chapter_title}, keeping folder")
            else:
                log(f"Skipping CBZ creation for {chapter_title} - only {downloaded_count} image(s) downloaded")
                # Remove temp folder
                shutil.rmtree(chapter_folder)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()