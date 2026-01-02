#!/usr/bin/env python3
"""
ManhuaFast scraper for MAGI.

Downloads manga/manhwa/manhua from manhuafast.net.
"""

# Standard library imports
import os
import re
import shutil
import sys
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
    get_image_extension,
    log,
    sanitize_title,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'ManhuaFast'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[ManhuaFast]')
ALLOWED_DOMAINS = ['manhuafast.net', 'cdn.manhuafast.net']
BASE_URL = 'https://manhuafast.net'


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from the manga listing page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    url = f"{BASE_URL}/manga/page/{page_num}/"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for "next" link
    is_last_page = 'next page-numbers' not in html and 'Next' not in html
    
    # Extract series URLs - look for manga entry links
    series_urls = re.findall(r'href="https://manhuafast\.net(/manga/[^/]+/)"', html)
    # Filter out chapter URLs and other non-series URLs
    series_urls = [url for url in series_urls if 'chapter' not in url and 'feed' not in url and 'genre' not in url]
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
    import html as html_module
    
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        title = html_module.unescape(title_match.group(1))
        title = title.replace(' – MANHUAFAST.NET', '').replace(' - MANHUAFAST.NET', '').strip()
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
    
    # First get the manga page to extract the post ID or other needed data
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract manga slug from URL
    manga_slug = manga_url.strip('/').split('/')[-1]
    
    # Try to get chapters via AJAX
    ajax_url = f"{BASE_URL}{manga_url}ajax/chapters/?t=1"
    try:
        ajax_response = session.post(ajax_url, timeout=30)
        ajax_response.raise_for_status()
        ajax_html = ajax_response.text
        
        # Extract chapter URLs from AJAX response
        chapter_urls = re.findall(r'href="https://manhuafast\.net(/manga/' + re.escape(manga_slug) + r'/chapter-[^/]+/)"', ajax_html)
        
        if chapter_urls:
            # Remove duplicates and sort by chapter number
            unique_urls = sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
            return unique_urls
    except Exception as e:
        warn(f"AJAX chapter loading failed: {e}, falling back to HTML parsing")
    
    # Fallback: extract from the original HTML (for sites that don't use AJAX)
    chapter_urls = re.findall(r'href="https://manhuafast\.net(' + re.escape(manga_url.rstrip('/')) + r'/chapter-[^/]+/)"', html)
    # Remove duplicates and sort by chapter number
    unique_urls = sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
    return unique_urls


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs for a given chapter.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs
    """
    full_url = f"{BASE_URL}{chapter_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\0', '')
    
    # Look for img src attributes that contain manga images
    image_urls = re.findall(r'<img[^>]*src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
    # Clean URLs and filter for manga images from cdn.manhuafast.net
    cleaned_urls = []
    for url in image_urls:
        url = url.strip()
        url = re.sub(r'https:///+', 'https://', url)
        if url.startswith('//'):
            url = 'https:' + url
        elif url.startswith('/'):
            url = 'https://cdn.manhuafast.net' + url
        elif not url.startswith('http'):
            continue
        if ('cdn.manhuafast.net' in url or 'WP-manga' in url) and 'thumbnails' not in url:
            cleaned_urls.append(url)
    return list(dict.fromkeys(cleaned_urls))  # unique


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting ManhuaFast scraper")
    log("Mode: Full Downloader")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        session = requests.Session()
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

    # Process series page by page to save progress incrementally
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
                    warn(f"Could not extract title for {series_url}, skipping")
                    continue
                
                title = sanitize_title(title)
                series_folder = os.path.join(FOLDER, f"{title} {DEFAULT_SUFFIX}")
                Path(series_folder).mkdir(parents=True, exist_ok=True)
                
                chapter_urls = extract_chapter_urls(session, series_url)
                if not chapter_urls:
                    warn(f"No chapters found for {title}")
                    continue
                
                log(f"Found {len(chapter_urls)} chapters for {title}")
                
                # Process each chapter
                for chapter_url in chapter_urls:
                    chapter_num_match = re.search(r'chapter-(\d+)', chapter_url)
                    if not chapter_num_match:
                        warn(f"Could not extract chapter number from {chapter_url}, skipping")
                        continue
                    
                    chapter_num = chapter_num_match.group(1)
                    chapter_folder = Path(series_folder) / f"{title} Ch.{chapter_num}"
                    
                    if os.path.exists(chapter_folder) and os.listdir(chapter_folder):
                        log(f"Chapter {chapter_num} already exists, skipping")
                        continue
                    
                    Path(chapter_folder).mkdir(parents=True, exist_ok=True)
                    
                    image_urls = extract_image_urls(session, chapter_url)
                    if not image_urls:
                        warn(f"No images found for chapter {chapter_num}")
                        continue
                    
                    log(f"Downloading {len(image_urls)} images for chapter {chapter_num}")
                    
                    # Download images
                    downloaded_files = []
                    for i, img_url in enumerate(image_urls, 1):
                        try:
                            img_response = session.get(img_url, timeout=30)
                            img_response.raise_for_status()
                            
                            # Determine file extension
                            content_type = img_response.headers.get('content-type', '')
                            if 'webp' in content_type:
                                ext = 'webp'
                            elif 'png' in content_type:
                                ext = 'png'
                            else:
                                ext = 'jpg'
                            
                            filename = f"{i:03d}.{ext}"
                            filepath = Path(chapter_folder) / filename
                            
                            if not DRY_RUN:
                                with open(filepath, 'wb') as f:
                                    f.write(img_response.content)
                            
                            downloaded_files.append(filepath)
                            
                        except Exception as e:
                            error(f"Failed to download image {img_url}: {e}")
                    
                    # Convert to WebP if enabled
                    if CONVERT_TO_WEBP and not DRY_RUN:
                        for filepath in downloaded_files:
                            if filepath.suffix.lower() in ('.jpg', '.jpeg', '.png'):
                                convert_to_webp(filepath)
                    
                    # Create CBZ if not dry run
                    if not DRY_RUN:
                        cbz_name = f"{title} Ch.{chapter_num}"
                        if create_cbz(chapter_folder, cbz_name):
                            # Remove temp folder
                            shutil.rmtree(chapter_folder)
                        else:
                            warn(f"CBZ creation failed for chapter {chapter_num}, keeping folder")
                    
                    total_chapters += 1
                    success(f"Completed chapter {chapter_num} for {title}")
                
                total_series += 1
                success(f"Completed series: {title}")
            
            if is_last_page:
                log("Reached last page, stopping.")
                break
            
            page += 1
            
        except Exception as e:
            error(f"Error processing page {page}: {e}")
            break
    
    success(f"Scraping completed. Processed {total_series} series and {total_chapters} chapters.")


if __name__ == '__main__':
    main()