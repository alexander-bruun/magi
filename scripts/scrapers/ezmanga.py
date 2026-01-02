#!/usr/bin/env python3
"""
EzManga scraper for MAGI.

Downloads manga/manhwa/manhua from ezmanga.org using API and browser automation.
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
    calculate_padding_width,
    convert_to_webp,
    create_cbz,
    check_duplicate_series,
    get_priority_config,
    encode_image_url,
    encode_url_path,
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'EzManga'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[EzManga]')
ALLOWED_DOMAINS = ['media.ezmanga.org', 'storage.ezmanga.org']
API_BASE = os.getenv('api', 'https://vapi.ezmanga.org')
BASE_URL = 'https://ezmanga.org'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('ezmanga')
EZMANGA_CF_CLEARANCE = os.getenv('EZMANGA_CF_CLEARANCE')


# =============================================================================
# Authentication & Browser Helpers
# =============================================================================
def get_auth_cookies(bypass_cookies=None):
    """
    Get authentication cookies from environment variables or browser.

    Args:
        bypass_cookies: Fallback cookies if environment vars not set

    Returns:
        dict: Cookies dictionary
    """
    # For EzManga, we need all cookies from browser session
    cf_clearance = os.getenv('EZMANGA_CF_CLEARANCE')
    
    if cf_clearance:
        # URL decode the cf_clearance cookie
        cf_clearance = urllib.parse.unquote(cf_clearance)
        return {'cf_clearance': cf_clearance}
    
    # Try to get fresh cookies using browser automation
    log("No environment cookies found, trying browser automation...")
    fresh_cookies = get_fresh_cookies()
    if fresh_cookies:
        log(f"Got {len(fresh_cookies)} fresh cookies from browser")
        return fresh_cookies
    
    # Fall back to bypass cookies
    if bypass_cookies:
        return bypass_cookies
    
    # No cookies available
    return {}


async def get_fresh_cookies():
    """
    Get fresh cookies by visiting the site with browser automation.

    Returns:
        dict: Cookies dictionary from browser session
    """
    try:
        async with AsyncCamoufox(
            headless=True,
            geoip=True,
            humanize=False,
            i_know_what_im_doing=True,
            config={'forceScopeAccess': True},
            disable_coop=True
        ) as browser:
            page = await browser.new_page()
            
            # Visit main site
            await page.goto(BASE_URL, wait_until='load', timeout=60000)
            
            # Solve Cloudflare captcha if present
            captcha_success = await solve_captcha(
                page, captcha_type='cloudflare', challenge_type='interstitial'
            )
            if not captcha_success:
                warn("Captcha solving may have failed, but continuing...")
            
            # Wait for cookies to be set
            await page.wait_for_timeout(2000)
            
            # Get all cookies
            cookies = {}
            for cookie in await page.context.cookies():
                cookies[cookie['name']] = cookie['value']
            
            return cookies
            
    except Exception as e:
        error(f"Failed to get fresh cookies: {e}")
        return {}


async def get_api_data_with_browser(url, page=None):
    """
    Get API data by navigating to the URL with browser automation.

    Args:
        url: API URL to fetch
        page: Optional existing page instance (with Cloudflare already bypassed)

    Returns:
        dict: Parsed JSON data or None
    """
    owns_browser = page is None
    browser = None
    
    try:
        if owns_browser:
            browser = await AsyncCamoufox(
                headless=True,
                geoip=True,
                humanize=False,
                i_know_what_im_doing=True,
                config={'forceScopeAccess': True},
                disable_coop=True
            ).__aenter__()
            page = await browser.new_page()
            
            # Visit main site first to establish session and bypass Cloudflare
            log("Navigating to main site for Cloudflare bypass...")
            await page.goto(BASE_URL, wait_until='load', timeout=60000)
            
            # Solve captcha if present
            captcha_success = await solve_captcha(
                page, captcha_type='cloudflare', challenge_type='interstitial'
            )
            if not captcha_success:
                warn("Captcha solving may have failed, but continuing...")
            
            # Wait for any JavaScript to complete
            await page.wait_for_timeout(2000)
        
        # Navigate to the API URL
        log(f"Fetching API data...")
        await page.goto(url, wait_until='load', timeout=60000)
        await page.wait_for_timeout(1000)
        
        # Get the page content
        content = await page.content()
        
        # Extract JSON from <pre> tag (common in API responses)
        pre_match = re.search(r'<pre[^>]*>(.*?)</pre>', content, re.DOTALL | re.IGNORECASE)
        if pre_match:
            json_content = pre_match.group(1).strip()
            try:
                data = json.loads(json_content)
                if isinstance(data, dict):
                    return data
            except Exception as e:
                warn(f"JSON parse error from <pre> tag: {e}")
        
        # Fallback: try to parse the body text as JSON
        body_match = re.search(r'<body[^>]*>(.*?)</body>', content, re.DOTALL | re.IGNORECASE)
        if body_match:
            body_text = re.sub(r'<[^>]+>', '', body_match.group(1)).strip()
            try:
                data = json.loads(body_text)
                if isinstance(data, dict):
                    return data
            except:
                pass
        
        # Another fallback: look for raw JSON in content
        try:
            # Find the largest JSON object in the content
            json_start = content.find('{')
            if json_start >= 0:
                # Try to find matching brace
                depth = 0
                for i, char in enumerate(content[json_start:], start=json_start):
                    if char == '{':
                        depth += 1
                    elif char == '}':
                        depth -= 1
                        if depth == 0:
                            json_str = content[json_start:i+1]
                            data = json.loads(json_str)
                            if isinstance(data, dict):
                                return data
                            break
        except:
            pass
        
        warn("Could not extract JSON data from page")
        return None
                    
    except Exception as e:
        error(f"Browser automation failed: {e}")
        return None
    finally:
        if owns_browser and browser:
            await browser.__aexit__(None, None, None)


async def test_api_access(session):
    """
    Test if we can access the EzManga API using browser automation.

    Args:
        session: requests.Session object

    Returns:
        bool: True if API access works
    """
    test_url = f"{API_BASE}/api/query?page=1&perPage=1&orderBy=createdAt"
    
    try:
        # Use browser automation to test API access
        data = await get_api_data_with_browser(test_url)
        if data and isinstance(data, dict) and ('posts' in data or 'data' in data):
            return True
        else:
            log("Browser API test failed - no valid data returned")
            return False
    except Exception as e:
        log(f"Browser API test failed with exception: {e}")
        return False


# =============================================================================
# Series Extraction
# =============================================================================
async def extract_series_urls_browser(session, page_num):
    """
    Extract series data using browser automation.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, bool is_last_page)
    """
    url = f"https://vapi.ezmanga.org/api/query?page={page_num}&perPage=21&orderBy=createdAt"
    
    log(f"Fetching series from API page {page_num} using browser...")
    
    # Try browser automation to get the actual JSON data
    data = await get_api_data_with_browser(url)
    if data and isinstance(data, dict) and 'meta' in data and 'data' in data:
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


def retry_request(session, method, url, max_retries=MAX_RETRIES, base_delay=RETRY_DELAY, **kwargs):
    """
    Make a request with retry logic and exponential backoff.

    Args:
        session: requests.Session object
        method: HTTP method (get, post, etc.)
        url: URL to request
        max_retries: Maximum number of retry attempts
        base_delay: Base delay for exponential backoff
        **kwargs: Additional arguments for request

    Returns:
        requests.Response object
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


def extract_series_urls(session, page_num):
    """
    Extract series data from the API with pagination support.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, bool is_last_page)
    """
    url = f"{API_BASE}/api/query?page={page_num}&perPage=21&orderBy=createdAt"

    # Set the required headers from the curl command
    headers = {
        'accept': 'application/json, text/plain, */*',
        'accept-language': 'en-GB,en-US;q=0.9,en;q=0.8',
        'dnt': '1',
        'origin': 'https://ezmanga.org',
        'priority': 'u=1, i',
        'referer': 'https://ezmanga.org/',
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
        session: requests.Session object (unused)
        series_data: Series data dictionary

    Returns:
        str: Series title
    """
    return series_data.get('title', '')


# =============================================================================
# Chapter Extraction
# =============================================================================
async def extract_chapter_urls(session, series_data):
    """
    Extract chapter data from series API using browser automation.

    Args:
        session: requests.Session object
        series_data: Series data dictionary

    Returns:
        list: Chapter data dictionaries sorted by chapter number
    """
    post_id = series_data.get('id', '')
    series_slug = series_data.get('series_slug', '')
    
    if not post_id:
        return []
    
    # Use the new API endpoint for chapters
    url = f"{API_BASE}/api/v2/posts/{post_id}/chapters?page=1&perPage=99999999&sortOrder=desc&q="
    
    try:
        # Use browser automation to get chapter data
        chapter_data = await get_api_data_with_browser(url)
        
        if not chapter_data or 'data' not in chapter_data:
            warn(f"No chapter data found for series {series_slug}")
            return []
        
        chapters = []
        for chapter in chapter_data['data']:
            chapters.append({
                'id': chapter['id'],
                'number': chapter['number'],
                'slug': chapter['slug'],
                'series_slug': series_slug,
                'is_accessible': chapter.get('isAccessible', False),
                'is_locked': chapter.get('isLocked', False),
                'price': chapter.get('price', 0)
            })
        
        # Sort chapters by number
        chapters.sort(key=lambda x: x['number'])
        
        return chapters
        
    except Exception as e:
        error(f"Failed to get chapters for series {series_slug}: {e}")
        return []


async def extract_image_urls(session, chapter_data):
    """
    Extract image URLs from chapter page by scraping the HTML.

    Args:
        session: requests.Session object
        chapter_data: Chapter data dictionary

    Returns:
        list: Image URLs in reading order
    """
    series_slug = chapter_data.get('series_slug', '')
    chapter_slug = chapter_data.get('slug', '')
    
    if not series_slug or not chapter_slug:
        error(f"Missing series_slug or chapter_slug for chapter {chapter_data.get('number', 'unknown')}")
        return []
    
    # Construct the chapter URL
    chapter_url = f"https://ezmanga.org/series/{series_slug}/{chapter_slug}"
    
    try:
        # Use browser automation to visit the chapter page and extract images
        image_urls = await extract_images_from_page(chapter_url)
        return image_urls
        
    except Exception as e:
        error(f"Failed to extract images from chapter page {chapter_url}: {e}")
        return []


async def extract_images_from_page(chapter_url, page=None):
    """
    Extract image URLs from a chapter page using browser automation.

    Args:
        chapter_url: Full URL of the chapter page
        page: Optional existing page instance (with Cloudflare already bypassed)

    Returns:
        list: Image URLs in reading order
    """
    owns_browser = page is None
    browser = None
    
    try:
        if owns_browser:
            browser = await AsyncCamoufox(
                headless=True,
                geoip=True,
                humanize=False,
                i_know_what_im_doing=True,
                config={'forceScopeAccess': True},
                disable_coop=True
            ).__aenter__()
            page = await browser.new_page()
            
            # First bypass Cloudflare on main site
            await page.goto(BASE_URL, wait_until='load', timeout=60000)
            await solve_captcha(page, captcha_type='cloudflare', challenge_type='interstitial')
            await page.wait_for_timeout(2000)
        
        # Now visit the chapter page
        await page.goto(chapter_url, wait_until='domcontentloaded', timeout=60000)
        await page.wait_for_timeout(500)  # Brief wait for any dynamic content
        
        # Get page content for regex extraction
        content = await page.content()
        
        # Extract image URLs from img src attributes (handles URLs with spaces)
        # First, try extracting from img tags which is more reliable
        img_src_urls = re.findall(r'<img[^>]+src=["\']([^"\']+storage\.ezmanga\.org[^"\']+)["\']', content)
        
        # Also try the old pattern for any URLs not in img tags
        direct_urls = re.findall(r'https://storage\.ezmanga\.org/[^\s"\'<>]+\.(?:jpg|jpeg|png|webp|gif)', content)
        
        # Combine and deduplicate
        all_urls = img_src_urls + direct_urls
        
        # Filter to only image extensions and remove duplicates
        seen = set()
        unique_urls = []
        for url in all_urls:
            # Skip if not an image
            if not re.search(r'\.(?:jpg|jpeg|png|webp|gif)$', url, re.IGNORECASE):
                continue
            if url not in seen:
                seen.add(url)
                unique_urls.append(url)
        
        # Sort by filename to ensure correct order (01.webp, 02.webp, etc.)
        unique_urls.sort(key=lambda x: x.split('/')[-1])
        
        return unique_urls
            
    except Exception as e:
        error(f"Browser automation failed for {chapter_url}: {e}")
        return []
    finally:
        if owns_browser and browser:
            await browser.__aexit__(None, None, None)


async def extract_chapter_urls_with_page(session, series_data, page):
    """
    Extract chapter data from series API using existing browser page.

    Args:
        session: requests.Session object
        series_data: Series data dictionary
        page: Existing browser page with Cloudflare already bypassed

    Returns:
        list: Chapter data dictionaries sorted by chapter number
    """
    post_id = series_data.get('id', '')
    series_slug = series_data.get('series_slug', '')
    
    if not post_id:
        return []
    
    # Use the API endpoint for chapters
    url = f"{API_BASE}/api/v2/posts/{post_id}/chapters?page=1&perPage=99999999&sortOrder=desc&q="
    
    try:
        # Use the existing browser page
        chapter_data = await get_api_data_with_browser(url, page)
        
        if not chapter_data or 'data' not in chapter_data:
            warn(f"No chapter data found for series {series_slug}")
            return []
        
        chapters = []
        for chapter in chapter_data['data']:
            chapters.append({
                'id': chapter['id'],
                'number': chapter['number'],
                'slug': chapter['slug'],
                'series_slug': series_slug,
                'is_accessible': chapter.get('isAccessible', False),
                'is_locked': chapter.get('isLocked', False),
                'price': chapter.get('price', 0)
            })
        
        # Sort chapters by number
        chapters.sort(key=lambda x: x['number'])
        
        return chapters
        
    except Exception as e:
        error(f"Failed to get chapters for series {series_slug}: {e}")
        return []


async def extract_image_urls_with_page(session, chapter_data, page):
    """
    Extract image URLs from chapter page using existing browser page.

    Args:
        session: requests.Session object
        chapter_data: Chapter data dictionary
        page: Existing browser page with Cloudflare already bypassed

    Returns:
        list: Image URLs in reading order
    """
    series_slug = chapter_data.get('series_slug', '')
    chapter_slug = chapter_data.get('slug', '')
    
    if not series_slug or not chapter_slug:
        error(f"Missing series_slug or chapter_slug for chapter {chapter_data.get('number', 'unknown')}")
        return []
    
    # URL-encode special characters in slugs (especially apostrophes)
    series_slug_encoded = encode_url_path(series_slug, safe='-')
    chapter_slug_encoded = encode_url_path(chapter_slug, safe='-')
    
    # Construct the chapter URL
    chapter_url = f"https://ezmanga.org/series/{series_slug_encoded}/{chapter_slug_encoded}"
    
    try:
        # Use existing browser page
        image_urls = await extract_images_from_page(chapter_url, page)
        return image_urls
        
    except Exception as e:
        error(f"Failed to extract images from chapter page {chapter_url}: {e}")
        return []


# =============================================================================
# Main Entry Point
# =============================================================================
async def main_async():
    """Async main function for the scraper."""
    print("Main function called", file=sys.stderr, flush=True)
    log("Starting EzManga scraper")
    log("Mode: Full Downloader")
    
    # Create a single browser session for the entire scrape
    log("Starting browser and bypassing Cloudflare (one-time)...")
    browser = await AsyncCamoufox(
        headless=True,
        geoip=True,
        humanize=False,
        i_know_what_im_doing=True,
        config={'forceScopeAccess': True},
        disable_coop=True
    ).__aenter__()
    
    try:
        page = await browser.new_page()
        
        # Bypass Cloudflare once at startup
        await page.goto(BASE_URL, wait_until='load', timeout=60000)
        captcha_success = await solve_captcha(
            page, captcha_type='cloudflare', challenge_type='interstitial'
        )
        if not captcha_success:
            warn("Captcha solving may have failed, but continuing...")
        await page.wait_for_timeout(2000)
        
        # Get cookies from browser session
        cookies = {}
        for cookie in await page.context.cookies():
            cookies[cookie['name']] = cookie['value']
        
        log(f"Got {len(cookies)} cookies from browser")
        session = get_session(cookies)
        
        success("Health check passed")
        success("Cloudflare bypassed (session will be reused)")
        
        # Collect all series using the existing browser session
        all_series_data = []
        page_num = 1
        
        while True:
            url = f"{API_BASE}/api/query?page={page_num}&perPage=99999999&orderBy=createdAt"
            log(f"Fetching series page {page_num}...")
            
            data = await get_api_data_with_browser(url, page)
            if not data:
                log(f"No data returned at page {page_num}")
                break
            
            # API can return either 'posts' or 'data' depending on endpoint
            posts = data.get('posts') or data.get('data', [])
            if not posts:
                log(f"No series found at page {page_num}")
                break
            
            total_count = data.get('totalCount') or data.get('meta', {}).get('total', 0)
            
            for post in posts:
                # Convert post to series format - handle both API response formats
                series_data = {
                    'id': post.get('id', ''),
                    'title': post.get('title') or post.get('postTitle', ''),
                    'series_slug': post.get('series_slug') or post.get('slug', ''),
                    'thumbnail': post.get('thumbnail') or post.get('featuredImage', ''),
                    'status': post.get('status') or post.get('seriesStatus', ''),
                    'badge': post.get('badge') or post.get('hot', False),
                    'chapters': post.get('chapters', []),
                    'paid_chapters': post.get('paid_chapters', []),
                    'free_chapters': post.get('free_chapters', [])
                }
                all_series_data.append(series_data)
            
            # Since we're getting all data in one request, we can break after the first page
            break
        
        success(f"Found {len(all_series_data)} series total")

        total_series = len(all_series_data)
        total_chapters = 0

        # Process each series
        for series_data in all_series_data:
            series_slug = series_data.get('series_slug', '')
            title = series_data.get('title', '')
            
            if not title:
                error(f"Could not extract title for series {series_slug}, skipping...")
                continue

            clean_title = sanitize_title(title)
            log(f"Processing: {clean_title}")

            # Check for duplicate in higher priority providers
            if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
                continue

            # Get chapters from the series data (reuse browser page)
            chapters = await extract_chapter_urls_with_page(session, series_data, page)
            if not chapters:
                warn(f"No chapters found for {title}, skipping...")
                continue

            # Create series directory (only after confirming chapters exist)
            series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
            if not DRY_RUN:
                series_directory.mkdir(parents=True, exist_ok=True)

            # Determine padding width
            chapter_numbers = [chapter.get('number', 0) for chapter in chapters]
            if chapter_numbers:
                max_chapter = max(chapter_numbers)
                padding_width = calculate_padding_width(int(max_chapter))
            else:
                padding_width = 2

            log(f"Found {len(chapters)} chapters (max: {max_chapter}, padding: {padding_width})")

            # Scan existing CBZ files to determine which chapters are already downloaded
            existing_chapters = get_existing_chapters(series_directory)
            log_existing_chapters(existing_chapters)

            consecutive_skips = 0
            for chapter in chapters:
                chapter_num = chapter.get('number', 0)
                chapter_slug = chapter.get('slug', '')
                is_accessible = chapter.get('is_accessible', False)
                is_locked = chapter.get('is_locked', False)
                price = chapter.get('price', 0)

                # Skip if chapter already exists
                if chapter_num in existing_chapters:
                    continue

                # Skip locked/premium chapters
                if is_locked or not is_accessible or price > 0:
                    log(f"Skipping: Chapter {chapter_num} (locked/premium)")
                    continue

                total_chapters += 1

                chapter_name = format_chapter_name(clean_title, chapter_num, padding_width, DEFAULT_SUFFIX)

                try:
                    image_urls = await extract_image_urls_with_page(session, chapter, page)
                except Exception as e:
                    error(f"Error extracting images for chapter {chapter_num}: {e}")
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

                # Create chapter directory
                chapter_folder = series_directory / chapter_name
                if not DRY_RUN:
                    chapter_folder.mkdir(parents=True, exist_ok=True)

                # Download images
                downloaded_count = 0
                for i, img_url in enumerate(image_urls, 1):
                    if not img_url:
                        continue
                    # URL-encode special characters (handles spaces in filenames)
                    img_url = encode_image_url(img_url)
                    ext = get_image_extension(img_url, 'jpg')
                    filename = chapter_folder / f"{i:03d}.{ext}"
                    try:
                        response = retry_request(session, 'get', img_url, timeout=30)
                        if not DRY_RUN:
                            with open(filename, 'wb') as f:
                                f.write(response.content)
                        downloaded_count += 1
                        if CONVERT_TO_WEBP and ext != 'webp':
                            convert_to_webp(filename)
                    except Exception as e:
                        warn(f"Failed to download image {i}: {e}")

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
                    if chapter_folder.exists():
                        shutil.rmtree(chapter_folder)

        log(f"Total series processed: {total_series}")
        log(f"Total chapters downloaded: {total_chapters}")
        success(f"Completed! Output: {FOLDER}")
    
    finally:
        # Always close the browser
        await browser.__aexit__(None, None, None)


def main():
    """Main entry point - synchronous wrapper for the async main function."""
    asyncio.run(main_async())


if __name__ == '__main__':
    main()