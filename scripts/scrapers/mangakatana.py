#!/usr/bin/env python3
"""
MangaKatana scraper for MAGI.

Downloads manga/manhwa/manhua from mangakatana.com.
"""

# Standard library imports
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'MangaKatana'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[MangaKatana]')
ALLOWED_DOMAINS = ['mangakatana.com', 'i1.mangakatana.com', 'i2.mangakatana.com', 'i3.mangakatana.com', 'i4.mangakatana.com', 'i5.mangakatana.com', 'i6.mangakatana.com', 'i7.mangakatana.com']
BASE_URL = 'https://mangakatana.com'


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    if page_num == 1:
        url = f"{BASE_URL}/latest"
    else:
        url = f"{BASE_URL}/latest/page/{page_num}"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for pagination
    # MangaKatana shows "next" link if there are more pages
    is_last_page = 'next page-numbers' not in html
    
    # Extract series URLs - look for full manga URLs
    series_urls = re.findall(r'href="(https://mangakatana\.com/manga/[^"]+\.\d+)"', html)
    # Convert to relative URLs for consistency
    series_urls = [url.replace('https://mangakatana.com', '') for url in series_urls]
    # Filter out chapter URLs (those containing /c)
    series_urls = [url for url in series_urls if '/c' not in url]
    return sorted(set(series_urls)), is_last_page


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    url = f"{BASE_URL}{series_url}"
    
    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract title from various possible locations
            # Try title tag first
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' - MangaKatana', '').strip()
                return title
            
            # Try h1 tag
            h1_match = re.search(r'<h1[^>]*>([^<]+)</h1>', html)
            if h1_match:
                return h1_match.group(1).strip()
                
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(f"Failed to extract title (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
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
        series_url: Relative URL of the series

    Returns:
        list: Chapter URLs sorted by chapter number
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter links - they follow pattern https://mangakatana.com/manga/series.id/c123
    chapter_urls = re.findall(r'href="(https://mangakatana\.com/manga/[^"]+/c[\d.]+[^"]*)"', html)
    # Convert to relative URLs
    chapter_urls = [url.replace('https://mangakatana.com', '') for url in chapter_urls]
    
    # Sort by chapter number (handle decimals)
    def chapter_key(url):
        match = re.search(r'/c([\d.]+)', url)
        if match:
            return float(match.group(1))
        return 0
    
    chapter_urls.sort(key=chapter_key)
    return list(dict.fromkeys(chapter_urls))  # unique


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs
    """
    full_url = f"{BASE_URL}{chapter_url}"
    
    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract image URLs from the JavaScript array thzq
            # Look for var thzq=['url1','url2',...];
            thzq_match = re.search(r'var thzq=\[([^\]]+)\];', html)
            if thzq_match:
                # Extract URLs from the array
                array_content = thzq_match.group(1)
                image_urls = re.findall(r"'(https://[^']+)'", array_content)
            else:
                # Fallback to looking for img tags
                image_urls = re.findall(r'<img[^>]*src="([^"]*mangakatana\.com[^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
            
            # Filter for allowed domains and remove thumbnails
            filtered_urls = []
            for url in image_urls:
                if any(domain in url for domain in ALLOWED_DOMAINS) and 'thumbnail' not in url.lower():
                    filtered_urls.append(url)
            
            # Sort by filename number if present
            def sort_key(url):
                match = re.search(r'/(\d+)\.(jpg|jpeg|png|webp)', url)
                if match:
                    return int(match.group(1))
                return 0
            
            filtered_urls.sort(key=sort_key)
            return list(dict.fromkeys(filtered_urls))  # unique
            
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(f"Failed to extract images (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
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
    log("Starting MangaKatana scraper")
    log("Mode: Full Downloader")

    # Create session with custom headers
    session = get_session()
    session.headers.update({
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.5',
        'Accept-Encoding': 'gzip, deflate',
        'Connection': 'keep-alive',
        'Upgrade-Insecure-Requests': '1',
        'Sec-Fetch-Dest': 'document',
        'Sec-Fetch-Mode': 'navigate',
        'Sec-Fetch-Site': 'none',
        'Cache-Control': 'max-age=0',
    })

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Process series page by page
    page = 1
    total_series = 0
    total_chapters = 0
    while page <= 5:  # Only process first 5 pages for testing
        try:
            page_series, is_last_page = extract_series_urls(session, page)
            if not page_series:
                log(f"No series found on page {page}, stopping.")
                break
            log(f"Found {len(page_series)} series on page {page}")
            
            # Process each series on this page
            for series_url in page_series:
                total_series += 1
                log(f"Processing series {total_series}: {series_url}")

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
                max_chapter = max(int(re.search(r'/c(\d+)', url).group(1)) for url in chapter_urls)
                padding_width = calculate_padding_width(max_chapter)
                log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

                # Scan existing CBZ files to determine which chapters are already downloaded
                existing_chapters = get_existing_chapters(series_directory)
                log_existing_chapters(existing_chapters)

                consecutive_skips = 0
                for chapter_url in chapter_urls:
                    chapter_num_match = re.search(r'/c(\d+)', chapter_url)
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

                    log(f"Downloading: Chapter {chapter_num} [{len(image_urls)} images]")

                    # Create chapter directory - use chapter name for consistency with other scrapers
                    chapter_folder = series_directory / chapter_name
                    chapter_folder.mkdir(parents=True, exist_ok=True)
                    
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

            if is_last_page:
                log(f"Reached last page (page {page}).")
                break
            page += 1
        except Exception as e:
            error(f"Error processing page {page}: {e}")
            break


if __name__ == '__main__':
    main()