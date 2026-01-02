#!/usr/bin/env python3
"""
LuaComic scraper for MAGI.

Downloads manga/manhwa/manhua from luacomic.org.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import sys
import time
import urllib.parse
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'LuaComic'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[LuaComic]')
ALLOWED_DOMAINS = ['media.luacomic.org']
API_BASE = os.getenv('api', 'https://api.luacomic.org')
BASE_URL = 'https://luacomic.org'
LUACOMIC_SESSION = os.getenv('LUACOMIC_SESSION')
LUACOMIC_CF_CLEARANCE = os.getenv('LUACOMIC_CF_CLEARANCE')


# =============================================================================
# Authentication Helpers
# =============================================================================
def get_auth_cookies(bypass_cookies=None):
    """
    Get authentication cookies from bypass cookies or environment variables.

    Args:
        bypass_cookies: Optional dict of cookies from Cloudflare bypass

    Returns:
        dict: Authentication cookies
    """
    # First try to get from bypass cookies
    if bypass_cookies:
        ts_session = bypass_cookies.get('ts-session')
        cf_clearance = bypass_cookies.get('cf_clearance')
        if ts_session and cf_clearance:
            # URL decode the ts-session cookie
            ts_session = urllib.parse.unquote(ts_session)
            return {'ts-session': ts_session, 'cf_clearance': cf_clearance}
    
    # Fall back to environment variables
    session_cookie = os.getenv('LUACOMIC_SESSION')
    cf_clearance = os.getenv('LUACOMIC_CF_CLEARANCE')
    
    if session_cookie and cf_clearance:
        # URL decode the ts-session cookie
        session_cookie = urllib.parse.unquote(session_cookie)
        return {'ts-session': session_cookie, 'cf_clearance': cf_clearance}
    
    # No cookies available
    return {}


def retry_request(session, method, url, max_retries=MAX_RETRIES, base_delay=1, **kwargs):
    """
    Retry a request with exponential backoff for rate limiting.

    Args:
        session: requests.Session object
        method: HTTP method (get, post, etc.)
        url: URL to request
        max_retries: Maximum number of retries
        base_delay: Base delay for exponential backoff
        **kwargs: Additional arguments for the request

    Returns:
        requests.Response object

    Raises:
        requests.exceptions.RequestException: If all retries fail
    """
    for attempt in range(max_retries):
        try:
            response = getattr(session, method.lower())(url, **kwargs)
            response.raise_for_status()
            return response
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 429:  # Too Many Requests
                if attempt < max_retries - 1:
                    delay = base_delay * (2 ** attempt)  # Exponential backoff
                    warn(f"Rate limited (429). Retrying in {delay}s... (attempt {attempt + 1}/{max_retries})")
                    time.sleep(delay)
                    continue
                else:
                    error(f"Rate limited after {max_retries} attempts: {url}")
                    raise
            else:
                # Other HTTP errors, don't retry
                raise
        except requests.exceptions.RequestException as e:
            if attempt < max_retries - 1:
                delay = base_delay * (2 ** attempt)
                warn(f"Request failed: {e}. Retrying in {delay}s... (attempt {attempt + 1}/{max_retries})")
                time.sleep(delay)
                continue
            else:
                error(f"Request failed after {max_retries} attempts: {url}")
                raise


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series data from the API with pagination support.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series data dicts, bool is_last_page)
    """
    url = f"{API_BASE}/query?page={page_num}&perPage=20&series_type=Comic&query_string=&orderBy=created_at&adult=true&status=All&tags_ids=%5B%5D"
    
    # Set the required headers from the curl command
    headers = {
        'accept': 'application/json, text/plain, */*',
        'accept-language': 'en-GB,en-US;q=0.9,en;q=0.8',
        'dnt': '1',
        'origin': 'https://luacomic.org',
        'priority': 'u=1, i',
        'referer': 'https://luacomic.org/',
        'sec-ch-ua': '"Chromium";v="143", "Not A(Brand";v="24"',
        'sec-ch-ua-mobile': '?0',
        'sec-ch-ua-platform': '"Windows"',
        'sec-fetch-dest': 'empty',
        'sec-fetch-mode': 'cors',
        'sec-fetch-site': 'same-site',
        'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36'
    }
    
    log(f"Fetching series from API page {page_num}...")
    response = retry_request(session, 'get', url, headers=headers, timeout=60)
    
    data = response.json()
    meta = data.get('meta', {})
    series_list = data.get('data', [])
    
    series_data = []
    for series in series_list:
        series_data.append({
            'id': series['id'],
            'title': series['title'],
            'series_slug': series['series_slug'],
            'thumbnail': series['thumbnail'],
            'status': series['status'],
            'badge': series['badge'],
            'paid_chapters': series.get('paid_chapters', []),
            'free_chapters': series.get('free_chapters', [])
        })
    
    current_page = meta.get('current_page', page_num)
    last_page = meta.get('last_page', page_num)
    is_last_page = current_page >= last_page
    
    log(f"Found {len(series_data)} series on page {page_num} (total pages: {last_page})")
    return series_data, is_last_page


def extract_series_title(session, series_data):
    """
    Extract series title from series data.

    Args:
        session: requests.Session object (unused but kept for consistency)
        series_data: Series data dict from API

    Returns:
        str: Series title
    """
    return series_data.get('title', '')


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_data):
    """
    Extract chapter data from the series data already fetched from API.

    Args:
        session: requests.Session object (unused but kept for consistency)
        series_data: Series data dict from API

    Returns:
        list: Chapter data dicts sorted by index
    """
    chapters = []
    
    # Add free chapters
    for chapter in series_data.get('free_chapters', []):
        chapters.append({
            'id': chapter['id'],
            'chapter_name': chapter['chapter_name'],
            'chapter_slug': chapter['chapter_slug'],
            'series_id': chapter['series_id'],
            'index': chapter.get('index', '0'),
            'is_premium': False
        })
    
    # Add paid chapters (but mark as premium so we can skip them)
    for chapter in series_data.get('paid_chapters', []):
        chapters.append({
            'id': chapter['id'],
            'chapter_name': chapter['chapter_name'],
            'chapter_slug': chapter['chapter_slug'],
            'series_id': chapter['series_id'],
            'index': chapter.get('index', '0'),
            'is_premium': True
        })
    
    # Sort chapters by index (assuming index represents chapter order)
    try:
        chapters.sort(key=lambda x: float(x.get('index', 0)))
    except (ValueError, TypeError):
        # If sorting fails, keep original order
        pass
    
    return chapters


def extract_image_urls(session, chapter_data, series_data):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_data: Chapter data dict
        series_data: Series data dict

    Returns:
        list: Image URLs
    """
    series_slug = series_data.get('series_slug')
    chapter_slug = chapter_data.get('chapter_slug')
    chapter_url = f"https://luacomic.org/series/{series_slug}/{chapter_slug}"
    
    response = retry_request(session, 'get', chapter_url, timeout=60)
    html = response.text
    
    # Check if premium
    if chapter_data.get('is_premium', False):
        return []
    
    # Extract image URLs from src attributes
    image_urls = re.findall(r'src="https://media\.luacomic\.org/file/[^"]+', html)
    
    # Remove src=" prefix
    image_urls = [url.replace('src="', '') for url in image_urls]
    
    # Remove thumbnail if present
    thumbnail = series_data.get('thumbnail', '')
    if thumbnail:
        image_urls = [url for url in image_urls if url != thumbnail]
    
    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting LuaComic scraper")
    log("Mode: Full Downloader")

    # Cloudflare bypass and authentication setup for API access
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies or 'cf_clearance' not in cookies:
            warn("Cloudflare bypass failed, using fallback cookies...")
            # Use fallback cookies from environment variables
            cookies = get_auth_cookies()
            session = get_session(cookies)
        else:
            # Use bypass cookies directly (they should contain both cf_clearance and ts-session)
            # If bypass cookies don't have ts-session, try to get it from auth cookies
            auth_cookies = get_auth_cookies(cookies)
            if auth_cookies:
                cookies.update(auth_cookies)
            session = get_session(cookies, headers)
            log("Cloudflare bypass successful")
            log(f"Obtained cookies: {list(cookies.keys())}")
    except Exception as e:
        warn(f"Cloudflare bypass failed: {e}, using fallback cookies...")
        cookies = get_auth_cookies()
        session = get_session(cookies)

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Fetch all series data with pagination
    all_series = []
    page_num = 1
    while True:
        try:
            series_list, is_last_page = extract_series_urls(session, page_num)
            all_series.extend(series_list)
            if is_last_page:
                break
            page_num += 1
        except Exception as e:
            error(f"Error fetching page {page_num}: {e}")
            break
    
    log(f"Found {len(all_series)} total series")
    
    total_series = len(all_series)
    total_chapters = 0

    # Process each series
    for series_data in all_series:
        series_id = series_data.get('id')
        title = extract_series_title(session, series_data)
        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")

        # Fetch chapters
        try:
            chapters = extract_chapter_urls(session, series_data)
        except Exception as e:
            error(f"Error fetching chapters for {title}: {e}")
            continue

        if not chapters:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Determine max chapter number for padding
        chapter_numbers = []
        for chapter in chapters:
            chapter_name = chapter.get('chapter_name', '')
            # Extract number from "Chapter X" format
            match = re.search(r'Chapter (\d+)', chapter_name)
            if match:
                chapter_numbers.append(int(match.group(1)))
            else:
                # Try to get from index for free chapters
                index = chapter.get('index')
                if index and isinstance(index, (int, float)):
                    chapter_numbers.append(int(float(index)))
        
        if chapter_numbers:
            max_chapter = max(chapter_numbers)
            padding_width = calculate_padding_width(max_chapter)
        else:
            padding_width = 3  # Default padding
        
        log(f"Found {len(chapters)} chapters (max: {max_chapter if chapter_numbers else 'unknown'}, padding: {padding_width})")
        
        # Scan existing CBZ files
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)
        
        for chapter in chapters:
            try:
                chapter_name = chapter.get('chapter_name', '')
                chapter_slug = chapter.get('chapter_slug', '')
                
                # Extract chapter number
                match = re.search(r'Chapter (\d+)', chapter_name)
                if match:
                    chapter_num = int(match.group(1))
                else:
                    # Try index
                    index = chapter.get('index')
                    if index and isinstance(index, (int, float)):
                        chapter_num = int(float(index))
                    else:
                        warn(f"Could not extract chapter number from {chapter_name}, skipping...")
                        continue
                
                if chapter_num in existing_chapters:
                    continue
                
                chapter_name_full = format_chapter_name(clean_title, chapter_num, padding_width, DEFAULT_SUFFIX)
                
                log(f"Processing: Chapter {chapter_num}")
                
                image_urls = extract_image_urls(session, chapter, series_data)
                if not image_urls:
                    if chapter.get('is_premium', False):
                        log(f"Skipping premium chapter: Chapter {chapter_num}")
                    else:
                        warn(f"No images found for Chapter {chapter_num}, skipping...")
                    continue
                
                log(f"Found {len(image_urls)} images")
                
                if DRY_RUN:
                    log(f"[DRY RUN] Would download {len(image_urls)} images for Chapter {chapter_num}")
                    continue
                
                log(f"Downloading: Chapter {chapter_num} [{len(image_urls)} images]")
                
                # Download images
                chapter_folder = series_directory / f"{chapter_name_full}"
                chapter_folder.mkdir(exist_ok=True)
                
                downloaded_count = 0
                for j, img_url in enumerate(image_urls):
                    try:
                        img_response = retry_request(session, 'get', img_url, timeout=60)
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
                if create_cbz(chapter_folder, chapter_name_full):
                    success(f"Created {chapter_name_full}.cbz ({downloaded_count} files)")
                
                # Clean up
                shutil.rmtree(chapter_folder)
                
            except Exception as e:
                error(f"Failed to process chapter {chapter.get('chapter_name', 'unknown')}: {e}")
            
            # Small delay between chapters to avoid rate limiting
            time.sleep(0.2)
        
        total_chapters += len(chapters)
        
        # Small delay between series to avoid rate limiting
        time.sleep(0.5)
    
    log(f"Processed {total_series} series with {total_chapters} total chapters")


if __name__ == '__main__':
    main()