#!/usr/bin/env python3
import json
"""
Kunmanga scraper for MAGI.

Downloads manga/manhwa/manhua from kunmanga.com.
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'Kunmanga'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[Kunmanga]')
ALLOWED_DOMAINS = ['kunmanga.com', 'kunsv1.com', 'manimg24.com']
BASE_URL = 'https://kunmanga.com'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('kunmanga')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page with pagination.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    url = f"{BASE_URL}/manga/page/{page_num}/?m_orderby=new-manga"
    response = session.get(url, timeout=30)
    
    # Check if page exists (404 means no more pages)
    if response.status_code == 404:
        return [], True  # is_last_page = True
    
    response.raise_for_status()
    html = response.text
    
    # Extract series URLs
    series_urls = re.findall(r'href="https://kunmanga\.com/manga/[^"]*/"', html)
    # Remove href=" and " and filter out chapter and feed URLs
    series_urls = [url.replace('href="', '').rstrip('"') for url in series_urls 
                   if '/chapter-' not in url and '/ch-' not in url and '/feed/' not in url and not url.endswith('/page/')]
    
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
                title = title_match.group(1).replace(' &#8211; Kunmanga', '').strip()
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
    chapter_urls = re.findall(r'href="https://kunmanga\.com/manga/[^"]*chapter-[^"]*/"', html)
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
    
    # Extract image URLs from chapter_preloaded_images script
    script_match = re.search(r'chapter_preloaded_images = (\[[\s\S]*?\])', html)
    if script_match:
        try:
            image_urls = json.loads(script_match.group(1))
            # Filter to only include chapter images (kunsv1.com or manimg24.com)
            image_urls = [url for url in image_urls if urlparse(url).netloc.endswith(('kunsv1.com', 'manimg24.com'))]
            # Remove duplicates while preserving order
            return list(dict.fromkeys(image_urls))
        except json.JSONDecodeError:
            pass
    
    # Fallback: Extract image URLs from data-src attributes (multiple domains)
    image_urls = re.findall(r'data-src="\s*(https?://(?:kunmanga\.com|sv\d*\.kunsv1\.com|h\d*\.manimg24\.com)/[^"]*\.(?:jpg|jpeg|png|webp))', html)
    # Filter to only include chapter images (kunsv1.com or manimg24.com)
    image_urls = [url for url in image_urls if urlparse(url).netloc.endswith(('kunsv1.com', 'manimg24.com'))]
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))
    
    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(r'src="\s*(https?://(?:kunmanga\.com|sv\d*\.kunsv1\.com|h\d*\.manimg24\.com)/[^"]*\.(?:jpg|jpeg|png|webp))', html)
        # Filter to only include chapter images (kunsv1.com or manimg24.com)
        image_urls = [url for url in image_urls if urlparse(url).netloc.endswith(('kunsv1.com', 'manimg24.com'))]
        image_urls = list(dict.fromkeys(image_urls))
    
    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Kunmanga scraper")
    log("Mode: Full Downloader")

    # Health check and bypass Cloudflare
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

    # Process series page by page, downloading as we go
    total_series = 0
    total_chapters = 0
    page = 1
    
    while True:
        try:
            page_series, is_last_page = extract_series_urls(session, page)
            if not page_series:
                log(f"No series found on page {page}, stopping.")
                break
            
            log(f"Found {len(page_series)} series on page {page}")
            
            # Process each series on this page immediately
            for series_url in page_series:
                log(f"Processing: {series_url}")

                title = extract_series_title(session, series_url)
                if not title:
                    error(f"Could not extract title for {series_url}, skipping...")
                    continue

                clean_title = sanitize_title(title)
                total_series += 1

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

                # Check if this series has any chapters with images by testing the first few
                has_images = False
                test_chapters = chapter_urls[:min(5, len(chapter_urls))]  # Test first 5 chapters
                for test_url in test_chapters:
                    try:
                        if extract_image_urls(session, test_url):
                            has_images = True
                            break
                    except Exception:
                        continue
                
                if not has_images:
                    log(f"No chapters with images found for {title}, skipping...")
                    continue

                # Create series directory (only after confirming chapters with images exist)
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
                            warn(f"CBZ creation failed for {chapter_title}, keeping folder")
                    else:
                        log(f"Skipping CBZ creation for {chapter_title} - only {downloaded_count} image(s) downloaded")
                        # Remove temp folder
                        shutil.rmtree(chapter_folder)

            if is_last_page:
                log(f"Reached last page (page {page}).")
                break
            page += 1
            
        except Exception as e:
            error(f"Error processing page {page}: {e}")
            break

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()