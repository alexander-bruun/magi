#!/usr/bin/env python3
import json
"""
GenzUpdates scraper for MAGI.

Downloads manga/manhwa/manhua from genzupdates.com.
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
    MAX_RETRIES,
    RETRY_DELAY,
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'GenzUpdates'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[GenzUpdates]')
ALLOWED_DOMAINS = ['cdn.meowing.org']
BASE_URL = 'https://genzupdates.com'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('genzupdates')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page with pagination.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    if page_num == 1:
        url = f"{BASE_URL}/series"
    else:
        url = f"{BASE_URL}/series/page/{page_num}/"
    
    log(f"Fetching series list from page {page_num}...")
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page
    is_last_page = 'next page-numbers' not in html and 'Next' not in html and '>' not in html
    
    # Extract series URLs
    series_urls = re.findall(r'href="(/series/[^/]+/)"', html)
    
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
        title = title_match.group(1).replace(' - Genz Toon', '').strip()
        return title
    
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
        list: Chapter URLs sorted
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter URLs
    chapter_urls = re.findall(r'href="(/chapter/[^"]+)"', html)
    
    # Sort numerically by chapter number (extract last numeric part from URL)
    unique_urls = list(set(chapter_urls))
    def get_chapter_num(url):
        parts = url.split('-')
        for part in reversed(parts):
            if part.isdigit():
                return int(part)
        return 0
    unique_urls.sort(key=get_chapter_num)
    return unique_urls


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs in reading order
    """
    url = f"{BASE_URL}{chapter_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Look for the #pages div and extract img tags with uid attributes
    pages_div_match = re.search(r'<div[^>]*id="pages"[^>]*>(.*?)</div>', html, re.DOTALL | re.IGNORECASE)
    if pages_div_match:
        pages_html = pages_div_match.group(1)
        # Extract uid attributes from img tags
        uid_matches = re.findall(r'<img[^>]*uid="([^"]+)"[^>]*>', pages_html, re.IGNORECASE)
        
        image_urls = []
        for uid in uid_matches:
            if uid and len(uid.strip()) > 0:
                image_url = f"https://cdn.meowing.org/uploads/{uid}"
                image_urls.append(image_url)
        
        return image_urls
    
    # Fallback: look for image ID patterns in the entire HTML
    id_pattern = r'[A-Z][A-Za-z0-9]{9,11}'
    candidates = re.findall(id_pattern, html)
    
    # Filter candidates to ensure they have mixed case and numbers
    image_ids = []
    for candidate in candidates:
        has_upper = any(c.isupper() for c in candidate)
        has_lower = any(c.islower() for c in candidate)
        has_digit = any(c.isdigit() for c in candidate)
        if has_upper and has_lower and has_digit and len(candidate) >= 10:
            image_ids.append(candidate)
    
    # Remove duplicates while preserving order
    seen = set()
    unique_ids = []
    for img_id in image_ids:
        if img_id not in seen:
            seen.add(img_id)
            unique_ids.append(img_id)
    
    # Verify each ID forms a valid image URL
    valid_image_urls = []
    for img_id in unique_ids:
        test_url = f"https://cdn.meowing.org/uploads/{img_id}"
        try:
            img_resp = session.head(test_url, timeout=5)
            if img_resp.status_code == 200:
                valid_image_urls.append(test_url)
        except:
            pass
    
    return valid_image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting GenzUpdates scraper")
    
    # Cloudflare bypass
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies or 'cf_clearance' not in cookies:
            warn("Cloudflare bypass failed, trying without bypass...")
            session = get_session()
        else:
            session = get_session(cookies, headers)
    except Exception as e:
        warn(f"Cloudflare bypass failed: {e}, trying without bypass...")
        session = get_session()
    
    # Health check
    try:
        response = session.get(BASE_URL, timeout=30)
        response.raise_for_status()
        success("Health check passed")
    except Exception as e:
        warn(f"Health check failed: {e}, continuing anyway...")
    
    # Create output directory
    output_dir = Path(FOLDER)
    output_dir.mkdir(exist_ok=True)
    
    page_num = 1
    processed_series = 0
    
    while True:
        try:
            series_urls, is_last_page = extract_series_urls(session, page_num)
            if not series_urls:
                if is_last_page:
                    log("No more series found, finishing...")
                    break
                else:
                    page_num += 1
                    continue
            
            log(f"Found {len(series_urls)} series on page {page_num}")
            
            for series_url in series_urls:
                try:
                    title = extract_series_title(session, series_url)
                    if not title:
                        warn(f"Could not extract title for {series_url}, skipping...")
                        continue
                    
                    clean_title = sanitize_title(title)
                    if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS, PRIORITY):
                        log(f"Skipping {clean_title} due to duplicate in higher priority provider")
                        continue
                    log(f"Title: {title}")
                    
                    chapter_urls = extract_chapter_urls(session, series_url)
                    if not chapter_urls:
                        warn(f"No chapters found for {title}, skipping...")
                        continue
                    
                    # Create series directory (only after confirming chapters exist)
                    series_directory = output_dir / clean_title
                    series_directory.mkdir(exist_ok=True)
                    
                    # Extract chapter numbers for padding
                    chapter_nums = []
                    for url in chapter_urls:
                        # Extract chapter number from URL like /chapter/series-chapter/
                        parts = url.split('-')
                        if len(parts) >= 2:
                            try:
                                # The last part might be the chapter ID, try to find a number
                                for part in reversed(parts):
                                    if part.isdigit():
                                        chapter_nums.append(int(part))
                                        break
                            except ValueError:
                                continue
                    
                    if not chapter_nums:
                        # Fallback: use sequential numbering
                        chapter_nums = list(range(1, len(chapter_urls) + 1))
                    
                    max_chapter = max(chapter_nums)
                    padding_width = calculate_padding_width(max_chapter)
                    log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")
                    
                    # Scan existing CBZ files
                    existing_chapters = get_existing_chapters(series_directory, pattern=r'Chapter ([\d.]+)')
                    log_existing_chapters(existing_chapters)
                    
                    for i, chapter_url in enumerate(chapter_urls):
                        try:
                            chapter_num = chapter_nums[i] if i < len(chapter_nums) else i + 1
                            
                            if chapter_num in existing_chapters:
                                continue
                            
                            chapter_name = format_chapter_name(clean_title, chapter_num, padding_width, DEFAULT_SUFFIX)
                            
                            log(f"Processing: Chapter {chapter_num}")
                            
                            image_urls = extract_image_urls(session, chapter_url)
                            if not image_urls:
                                warn(f"No images found for Chapter {chapter_num}, skipping...")
                                continue
                            
                            log(f"Found {len(image_urls)} images")
                            
                            if DRY_RUN:
                                log(f"[DRY RUN] Would download {len(image_urls)} images for Chapter {chapter_num}")
                                continue
                            
                            log(f"Downloading: Chapter {chapter_num} [{len(image_urls)} images]")
                            
                            # Download images
                            chapter_folder = series_directory / f"{chapter_name}"
                            chapter_folder.mkdir(exist_ok=True)
                            
                            downloaded_count = 0
                            for j, img_url in enumerate(image_urls):
                                try:
                                    img_response = session.get(img_url, timeout=30)
                                    img_response.raise_for_status()
                                    
                                    # Determine file extension
                                    content_type = img_response.headers.get('content-type', '')
                                    if 'webp' in content_type:
                                        ext = '.webp'
                                    elif 'png' in content_type:
                                        ext = '.png'
                                    else:
                                        ext = '.jpg'
                                    
                                    img_filename = f"{j+1:03d}{ext}"
                                    img_path = chapter_folder / img_filename
                                    
                                    with open(img_path, 'wb') as f:
                                        f.write(img_response.content)
                                    
                                    downloaded_count += 1
                                    print(f"  [{j+1:03d}/{len(image_urls):03d}] {img_url} Success", file=sys.stderr, flush=True)
                                    
                                    # Convert to WebP if needed
                                    if CONVERT_TO_WEBP and ext != '.webp':
                                        try:
                                            convert_to_webp(img_path)
                                        except Exception as e:
                                            # WebP conversion failed, keep original file
                                            pass
                                    
                                except Exception as e:
                                    error(f"Failed to download image {j+1}: {e}")
                            
                            # Create CBZ
                            if create_cbz(chapter_folder, chapter_name):
                                success(f"Created {chapter_name}.cbz ({downloaded_count} files)")
                            
                            # Clean up
                            shutil.rmtree(chapter_folder)
                            
                        except Exception as e:
                            error(f"Failed to process chapter {i+1}: {e}")
                    
                    processed_series += 1
                    
                except Exception as e:
                    error(f"Failed to process series {series_url}: {e}")
            
            if is_last_page:
                break
            page_num += 1
            
        except Exception as e:
            error(f"Failed to process page {page_num}: {e}")
            break
    
    log(f"Processed {processed_series} series")


if __name__ == '__main__':
    main()