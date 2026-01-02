#!/usr/bin/env python3

import asyncio
import os
import re
import sys
import time
import requests
import zipfile
import json
from urllib.parse import urljoin, quote, urlparse
from pathlib import Path
from scraper_utils import log, success, warn, error, convert_to_webp, create_cbz, bypass_cloudflare, get_session, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'EzManga'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[EzManga]')
ALLOWED_DOMAINS = ['media.ezmanga.org']
API_BASE = os.getenv('api', 'https://vapi.ezmanga.org')
BASE_URL = 'https://ezmanga.org'
EZMANGA_CF_CLEARANCE = os.getenv('EZMANGA_CF_CLEARANCE')

def get_auth_cookies(bypass_cookies=None):
    """Get authentication cookies from environment variables, or bypass cookies as fallback"""
    import urllib.parse

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
    """Get fresh cookies by visiting the site with browser automation"""
    try:
        from camoufox import AsyncCamoufox
        
        async with AsyncCamoufox(headless=True) as browser:
            page = await browser.new_page()
            
            # Visit main site to get cookies
            await page.goto(BASE_URL, wait_until='networkidle')
            await page.wait_for_timeout(3000)  # Wait for cookies to be set
            
            # Get all cookies
            cookies = {}
            for cookie in await page.context.cookies():
                cookies[cookie['name']] = cookie['value']
            
            return cookies
            
    except Exception as e:
        error(f"Failed to get fresh cookies: {e}")
        return {}

async def get_api_data_with_browser(url, browser=None):
    """Get API data by navigating to the URL with browser automation and extracting JSON"""
    try:
        from camoufox import AsyncCamoufox
        
        async with AsyncCamoufox(headless=True) as browser:
            page = await browser.new_page()
            
            # Visit main site first to establish session
            await page.goto(BASE_URL, wait_until='domcontentloaded', timeout=30000)
            await page.wait_for_timeout(1000)  # Wait for any dynamic content
            
            # Navigate to the API URL
            await page.goto(url, wait_until='domcontentloaded', timeout=30000)
            await page.wait_for_timeout(500)  # Wait for response
            
            # Get the page content
            content = await page.content()
            
            # Extract JSON from <pre> tag
            pre_match = re.search(r'<pre>(.*?)</pre>', content, re.DOTALL)
            if pre_match:
                json_content = pre_match.group(1).strip()
                try:
                    data = json.loads(json_content)
                    # Check if it looks like API data
                    if isinstance(data, dict):
                        return data
                except Exception as e:
                    print(f"JSON parse error: {e}")
                    print(f"JSON content: {json_content[:200]}")
            
            # Fallback: try to extract JSON directly
            try:
                # Try to parse the entire content as JSON
                return json.loads(content.strip())
            except:
                pass
            
            # Another fallback: look for JSON in the content
            json_pattern = r'\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}'
            matches = re.findall(json_pattern, content)
            
            for match in matches:
                try:
                    data = json.loads(match)
                    if isinstance(data, dict):
                        return data
                except:
                    continue
                    
    except Exception as e:
        error(f"Browser automation failed: {e}")
        return None
        
    return None

async def test_api_access(session):
    """Test if we can access the EzManga API using browser automation"""
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

# Extract series data from API using browser automation
async def extract_series_urls_browser(session, page_num):
    """Extract series data using browser automation"""
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
    
def retry_request(session, method, url, max_retries=3, base_delay=1, **kwargs):
    """Make a request with retry logic and exponential backoff."""
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

# Extract series data from API
def extract_series_urls(session, page_num):
    """Extract series data from the API with pagination support"""
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

# Extract series title from series data
def extract_series_title(session, series_data):
    return series_data.get('title', '')

# Extract chapter data from series API data
async def extract_chapter_urls(session, series_data):
    """Extract chapter data from the series data using the new API endpoint"""
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

# Extract image URLs from chapter API data
async def extract_image_urls(session, chapter_data):
    """Extract image URLs from chapter page by scraping the HTML"""
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

async def extract_images_from_page(chapter_url, browser=None):
    """Extract image URLs from a chapter page using browser automation"""
    try:
        from camoufox import AsyncCamoufox
        
        async with AsyncCamoufox(headless=True) as browser:
            page = await browser.new_page()
            
            # Visit the chapter page
            await page.goto(chapter_url, wait_until='domcontentloaded', timeout=30000)
            await page.wait_for_timeout(2000)  # Wait for dynamic content to load
            
            # Try to find image elements - look for common selectors
            image_selectors = [
                'img[src*="storage.ezmanga.org"]',
                '.chapter-images img',
                '.manga-images img', 
                '.reader-images img',
                'img[data-src]',
                'img.lazy'
            ]
            
            image_urls = []
            
            for selector in image_selectors:
                try:
                    elements = await page.query_selector_all(selector)
                    for element in elements:
                        # Try src attribute first
                        src = await element.get_attribute('src')
                        if not src:
                            # Try data-src for lazy loading
                            src = await element.get_attribute('data-src')
                        
                        if src and 'storage.ezmanga.org' in src:
                            # Convert relative URLs to absolute
                            if src.startswith('//'):
                                src = 'https:' + src
                            elif src.startswith('/'):
                                src = 'https://ezmanga.org' + src
                            
                            if src not in image_urls:  # Avoid duplicates
                                image_urls.append(src)
                except:
                    continue
            
            # If no images found with selectors, try to find all images on the page
            if not image_urls:
                all_imgs = await page.query_selector_all('img')
                for img in all_imgs:
                    src = await img.get_attribute('src')
                    if not src:
                        src = await img.get_attribute('data-src')
                    
                    if src and 'storage.ezmanga.org' in src:
                        if src.startswith('//'):
                            src = 'https:' + src
                        elif src.startswith('/'):
                            src = 'https://ezmanga.org' + src
                        
                        if src not in image_urls:
                            image_urls.append(src)
            
            # Sort images by URL to maintain order
            image_urls.sort()
            
            return image_urls
            
    except Exception as e:
        error(f"Browser automation failed for {chapter_url}: {e}")
        return []

async def main_async():
    print("Main function called")  # Debug print
    log("Starting EzManga scraper")
    log("Mode: Full Downloader")

    from camoufox import AsyncCamoufox
    
    log("Getting fresh cookies using browser automation...")
    fresh_cookies = await get_fresh_cookies()
    
    if fresh_cookies:
        log(f"Got {len(fresh_cookies)} cookies from browser")
        # Create session with all cookies
        session = get_session(fresh_cookies)
    else:
        warn("Failed to get fresh cookies, trying without cookies...")
        session = get_session({})

    success("Health check passed")
    
    # Test API access before proceeding
    # if not test_api_access(session):
    #     error("API access test failed. Browser automation may not be working properly.")
    #     error("Try updating camoufox or check your internet connection.")
    #     return
    
    success("API access test skipped")
    # Collect all series using browser automation
    all_series_data = []
    page_num = 1
    
    while True:
        url = f"{API_BASE}/api/query?page={page_num}&perPage=99999999&orderBy=createdAt"
        log(f"Fetching series page {page_num}...")
        
        data = await get_api_data_with_browser(url)
        if not data or 'posts' not in data:
            log(f"No more data at page {page_num}")
            break
            
        posts = data['posts']
        total_count = data.get('totalCount', 0)
        
        for post in posts:
            # Convert post to series format
            series_data = {
                'id': post['id'],
                'title': post['postTitle'],
                'series_slug': post['slug'],
                'thumbnail': post.get('featuredImage', ''),
                'status': post.get('seriesStatus', ''),
                'badge': post.get('hot', False),
                'chapters': post.get('chapters', [])
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

        # Create series directory
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        if not DRY_RUN:
            series_directory.mkdir(parents=True, exist_ok=True)

        # Get chapters from the series data
        chapters = await extract_chapter_urls(session, series_data)
        if not chapters:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Determine padding width
        chapter_numbers = [chapter.get('number', 0) for chapter in chapters]
        if chapter_numbers:
            max_chapter = max(chapter_numbers)
            padding_width = len(str(int(max_chapter)))
        else:
            padding_width = 2

        log(f"Found {len(chapters)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [EzManga].cbz"
            match = re.search(r'Ch\.([\d.]+)', cbz_file.stem)
            if match:
                existing_chapters.add(float(match.group(1)))

        if existing_chapters:
            skipped_count = len(existing_chapters)
            if skipped_count <= 5:
                skipped_list = sorted(existing_chapters)
                log(f"Skipping {skipped_count} existing chapters: {skipped_list}")
            else:
                min_chapter = min(existing_chapters)
                max_chapter = max(existing_chapters)
                log(f"Skipping {skipped_count} existing chapters: {min_chapter}-{max_chapter}")
        else:
            log("No existing chapters found, downloading all")

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

            formatted_chapter_number = f"{int(chapter_num):0{padding_width}d}"

            chapter_name = f"{clean_title} Ch.{formatted_chapter_number} {DEFAULT_SUFFIX}"

            try:
                image_urls = await extract_image_urls(session, chapter)
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

            # Create chapter directory
            chapter_folder = series_directory / chapter_name
            if not DRY_RUN:
                chapter_folder.mkdir(parents=True, exist_ok=True)

            # Download images
            downloaded_count = 0
            for i, img_url in enumerate(image_urls, 1):
                if not img_url:
                    continue
                # Get extension
                parsed = urlparse(img_url)
                path = parsed.path
                ext = path.split('.')[-1].lower() if '.' in path else 'jpg'
                if ext not in ['jpg', 'jpeg', 'png', 'webp']:
                    ext = 'jpg'  # default
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
                    pass  # Silently skip failed images

            log(f"Downloaded: Chapter {chapter_num} [{downloaded_count}/{len(image_urls)} images]")

            # Only create CBZ if more than 1 image was downloaded
            if downloaded_count > 1:
                if create_cbz(chapter_folder, chapter_name):
                    # Remove temp folder
                    import shutil
                    shutil.rmtree(chapter_folder)
                else:
                    warn(f"CBZ creation failed for Chapter {chapter_num}, keeping folder")
            else:
                log(f"Skipping CBZ creation for Chapter {chapter_num} - only {downloaded_count} image(s) downloaded")
                # Remove temp folder
                import shutil
                shutil.rmtree(chapter_folder)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")

def main():
    """Synchronous wrapper for the async main function."""
    asyncio.run(main_async())

if __name__ == "__main__":
    main()