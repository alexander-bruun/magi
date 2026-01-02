#!/usr/bin/env python3
"""
MangaYY scraper for MAGI.

Downloads manga/manhwa/manhua from mangayy.org.
"""

# Standard library imports
import asyncio
import html as html_module
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
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'MangaYY'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[MangaYY]')
ALLOWED_DOMAINS = ['mangayy.org', 'yy.mangayy.org']
BASE_URL = 'https://mangayy.org'


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
    url = f"{BASE_URL}/page/{page_num}/?m_orderby=new-manga"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for "next" link
    is_last_page = 'next page-numbers' not in html and 'Next' not in html
    
    # Extract series URLs - look for manga entry links
    series_urls = re.findall(r'href="https://mangayy\.org(/manga/[^/]+/)"', html)
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
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        title = html_module.unescape(title_match.group(1))
        title = title.replace(' – MangaYY', '').strip()
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
        chapter_urls = re.findall(r'href="https://mangayy\.org(/manga/' + re.escape(manga_slug) + r'/chapter-[^/]+/)"', ajax_html)
        
        if chapter_urls:
            # Remove duplicates and sort by chapter number
            unique_urls = sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
            return unique_urls
    except Exception as e:
        warn(f"AJAX chapter loading failed: {e}, falling back to HTML parsing")
    
    # Fallback: extract from the original HTML (for sites that don't use AJAX)
    chapter_urls = re.findall(r'href="https://mangayy\.org(' + re.escape(manga_url.rstrip('/')) + r'/chapter-[^/]+/)"', html)
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
    # Clean URLs and filter for manga images from yy.mangayy.org
    cleaned_urls = []
    for url in image_urls:
        url = url.strip()
        if ('yy.mangayy.org' in url or 'WP-manga' in url) and 'thumbnails' not in url:
            # Remove any leading/trailing whitespace or encoded characters
            url = re.sub(r'^[^h]*', '', url)  # Remove anything before 'http'
            if url.startswith('http'):
                cleaned_urls.append(url)
    return list(dict.fromkeys(cleaned_urls))  # unique


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting MangaYY scraper")
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
                max_chapter = max(int(re.search(r'chapter-(\d+)', url).group(1)) for url in chapter_urls)
                padding_width = calculate_padding_width(max_chapter)
                log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

                # Scan existing CBZ files to determine which chapters are already downloaded
                existing_chapters = get_existing_chapters(series_directory)
                log_existing_chapters(existing_chapters)

                for chapter_url in chapter_urls:
                    chapter_num = int(re.search(r'chapter-(\d+)', chapter_url).group(1))

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
                    # Use a fresh session for image downloads (no Cloudflare bypass needed for image domain)
                    image_session = requests.Session()
                    image_session.headers.update({
                        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
                        'Referer': f"{BASE_URL}{chapter_url}"
                    })
                    for i, img_url in enumerate(image_urls, 1):
                        if not img_url or not any(domain in img_url for domain in ALLOWED_DOMAINS):
                            continue
                        ext = get_image_extension(img_url, 'jpg')
                        filename = chapter_folder / f"{i:03d}.{ext}"
                        try:
                            response = image_session.get(img_url, timeout=30)
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

                total_series += 1
                log(f"Completed series: {clean_title} ({len(chapter_urls)} chapters)")

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