#!/usr/bin/env python3
"""
StoneScape scraper for MAGI.

Downloads manga/manhwa/manhua from stonescape.xyz.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import sys
import time
from pathlib import Path
from urllib.parse import quote, urljoin

# Third-party imports
import requests

# Local imports
from scraper_utils import (
    MAX_RETRIES,
    RETRY_DELAY,
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'StoneScape'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[StoneScape]')
ALLOWED_DOMAINS = ['stonescape.xyz']
USER_AGENT = os.getenv('user_agent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36')
BASE_URL = 'https://stonescape.xyz'


# =============================================================================
# Utility Functions
# =============================================================================
def retry_request(url, session, max_retries=MAX_RETRIES, timeout=60):
    """
    Make a request with retry logic and exponential backoff.

    Args:
        url: URL to request
        session: requests.Session object
        max_retries: Maximum number of retry attempts
        timeout: Request timeout in seconds

    Returns:
        requests.Response object
    """
    for attempt in range(max_retries):
        try:
            response = session.get(url, timeout=timeout)
            if response.status_code == 429:
                wait_time = 2 ** attempt  # Exponential backoff: 1s, 2s, 4s
                warn(f"Rate limited (429). Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})")
                time.sleep(wait_time)
                continue
            response.raise_for_status()
            return response
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt
                warn(f"Request failed: {e}. Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})")
                time.sleep(wait_time)
            else:
                raise e


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
    if page_num == 1:
        url = "https://stonescape.xyz/series/"
    else:
        url = f"https://stonescape.xyz/series/page/{page_num}/"
    
    try:
        response = retry_request(url, session)
    except Exception as e:
        # If page doesn't exist, we've reached the end
        if "404" in str(e) or "Not Found" in str(e):
            return [], True
        raise
    
    html = response.text.replace('\n', '')
    
    # Extract series URLs from the listing
    series_urls = []
    
    # Look for href links to series
    href_pattern = r'href=\"(https://stonescape\.xyz/series/[^\"]+)\"'
    href_links = re.findall(href_pattern, html)
    
    # Filter to unique series (not chapters)
    series_set = set()
    for link in href_links:
        # Remove the base URL
        relative_link = link.replace('https://stonescape.xyz', '')
        # Check if it's a series link (not a chapter, not feed, not page)
        if ('/ch-' not in relative_link and 
            not relative_link.endswith('/series/') and
            '/feed' not in relative_link and
            '/page/' not in relative_link):
            series_set.add(relative_link)
    
    series_urls = sorted(list(series_set))
    
    # Check if there's a next page
    has_next_page = f'/series/page/{page_num + 1}/' in html or f'page/{page_num + 1}' in html
    
    return series_urls, not has_next_page  # is_last_page = True if no next page

def extract_series_title(session, series_url):
    """
    Extract series title from the series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    full_url = urljoin(BASE_URL, series_url)
    
    response = retry_request(full_url, session)
    html = response.text.replace('\n', '')
    
    # Try to extract title from various patterns
    title_patterns = [
        r'<h1[^>]*>([^<]*)</h1>',
        r'<title>([^|]*)\|',
        r'"title":"([^"]*)"',
        r'<meta property="og:title" content="([^"]*)"',
    ]
    
    for pattern in title_patterns:
        match = re.search(pattern, html, re.IGNORECASE)
        if match:
            title = match.group(1).strip()
            # Clean up common suffixes
            title = re.sub(r'\s*\|\s*StoneScape.*$', '', title, re.IGNORECASE)
            return title
    
    return None

# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from the series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: Sorted list of chapter URLs
    """
    full_url = urljoin(BASE_URL, series_url)
    
    # Get the series slug from the URL
    series_slug = series_url.strip('/').split('/')[-1]
    
    # Try to get chapters via AJAX endpoint
    ajax_url = f"{full_url}ajax/chapters/"
    
    chapter_urls = []
    
    try:
        # POST request to AJAX endpoint
        headers = {
            'accept': '*/*',
            'accept-language': 'en-GB,en-US;q=0.9,en;q=0.8',
            'content-length': '0',
            'dnt': '1',
            'origin': BASE_URL,
            'priority': 'u=1, i',
            'referer': full_url,
            'sec-ch-ua': '"Chromium";v="143", "Not A(Brand";v="24"',
            'sec-ch-ua-mobile': '?0',
            'sec-ch-ua-platform': '"Windows"',
            'sec-fetch-dest': 'empty',
            'sec-fetch-mode': 'cors',
            'sec-fetch-site': 'same-origin',
            'user-agent': USER_AGENT,
            'x-requested-with': 'XMLHttpRequest'
        }
        
        response = session.post(ajax_url, headers=headers, timeout=30)
        
        if response.status_code == 200:
            try:
                # Try to parse as JSON first
                try:
                    data = response.json()
                    if isinstance(data, dict) and 'data' in data:
                        html_content = data['data']
                    else:
                        html_content = response.text
                except:
                    # If not JSON, treat as HTML
                    html_content = response.text
                
                # Parse chapter links from the HTML content
                chapter_links = re.findall(r'href="([^"]*?/ch-[^"]*?)"', html_content)
                
                for link in chapter_links:
                    if f'/series/{series_slug}/' in link and '/ch-' in link:
                        # Convert to relative URL
                        if link.startswith('http'):
                            relative_url = link.replace(BASE_URL, '')
                        else:
                            relative_url = link
                        
                        if relative_url not in chapter_urls:
                            chapter_urls.append(relative_url)
                
                if chapter_urls:  # If we found chapters via AJAX, return them
                    # Sort chapters by number
                    def extract_chapter_num(url):
                        match = re.search(r'ch-(\d+)', url)
                        return int(match.group(1)) if match else 0
                    
                    chapter_urls.sort(key=extract_chapter_num)
                    return chapter_urls
                
            except Exception as e:
                pass  # Silently fail and try fallback
        
    except Exception as e:
        pass  # Silently fail and try fallback
    
    # Fallback: Try incremental chapter discovery
    chapter_urls = []
    max_chapters_to_check = 50  # Reasonable limit
    consecutive_empty = 0
    
    for chapter_num in range(1, max_chapters_to_check + 1):
        chapter_url = f"/series/{series_slug}/ch-{chapter_num}/"
        full_chapter_url = urljoin(BASE_URL, chapter_url)
        
        try:
            # GET request to check if chapter actually exists and has content
            response = session.get(full_chapter_url, timeout=10)
            if response.status_code == 200:
                html = response.text
                
                # Check if this is actually a valid chapter page
                # Look for manga ID and actual chapter content
                has_manga_id = 'manga_' in html
                has_chapter_indicator = 'ch-' in html or 'chapter' in html.lower()
                has_images = '<img' in html and ('data-src' in html or 'src=' in html)
                
                # More strict validation: must have manga ID AND images
                if has_manga_id and has_images:
                    chapter_urls.append(chapter_url)
                    consecutive_empty = 0
                else:
                    consecutive_empty += 1
            else:
                consecutive_empty += 1
            
            # Stop if we get 3 consecutive empty/invalid chapters
            if consecutive_empty >= 3:
                break
                
        except Exception as e:
            consecutive_empty += 1
            if consecutive_empty >= 3:
                break

    chapter_urls.sort(key=lambda url: int(re.search(r'ch-(\d+)', url).group(1)) if re.search(r'ch-(\d+)', url) else 0)

    return chapter_urls


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: List of image URLs, None if locked, empty list if not found
    """
    full_url = urljoin(BASE_URL, chapter_url)
    
    for attempt in range(MAX_RETRIES):
        try:
            response = retry_request(full_url, session)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text
            
            # Check if chapter is locked or unavailable
            locked_indicators = [
                "this chapter is locked",
                "chapter locked",
                "unlock this chapter", 
                "premium content",
                "members only"
            ]
            is_locked = any(indicator in html.lower() for indicator in locked_indicators)
            if is_locked:
                return None  # Locked/unavailable
            
            # Extract image URLs - try various patterns
            images = []
            
            # Look for img tags with src - focus on WP-manga chapter images
            img_matches = re.findall(r'<img[^>]*src=\"([^\"]+)\"[^>]*class=\"wp-manga-chapter-img\"', html, re.IGNORECASE | re.DOTALL)
            
            for img_url in img_matches:
                img_url = img_url.strip()  # Remove whitespace
                if any(img_url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif']):
                    # Filter out UI elements, logos, watermarks, etc.
                    skip_patterns = ['logo', 'banner', 'icon', 'button', 'watermark', 'placeholder', 'loading', 'avatar', 'thumb']
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        # Only include images that look like chapter pages (numbered files or manga_ in URL)
                        filename = img_url.split('/')[-1].split('.')[0]  # Get filename without extension
                        if re.match(r'^\d+$', filename) or 'manga_' in img_url or filename in ['0-black']:
                            images.append(img_url)
            
            # Also try the general img src pattern as fallback
            if not images:
                img_matches = re.findall(r'<img[^>]*src=\"([^\"]+)\"', html, re.IGNORECASE | re.DOTALL)
                
                for img_url in img_matches:
                    img_url = img_url.strip()
                    if any(img_url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif']):
                        skip_patterns = ['logo', 'banner', 'icon', 'button', 'watermark', 'placeholder', 'loading', 'avatar', 'thumb']
                        if not any(skip in img_url.lower() for skip in skip_patterns):
                            filename = img_url.split('/')[-1].split('.')[0]
                            if re.match(r'^\d+$', filename) or 'manga_' in img_url or filename in ['0-black']:
                                images.append(img_url)
            
            # Also look for data-src or similar lazy loading attributes
            data_src_matches = re.findall(r'data-src=\"([^\"]+)\"', html, re.IGNORECASE | re.DOTALL)
            
            for img_url in data_src_matches:
                img_url = img_url.strip()
                if any(img_url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif']):
                    skip_patterns = ['logo', 'banner', 'icon', 'button', 'watermark', 'placeholder', 'loading', 'avatar', 'thumb']
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        filename = img_url.split('/')[-1].split('.')[0]
                        if re.match(r'^\d+$', filename) or 'manga_' in img_url or filename in ['0-black']:
                            images.append(img_url)
            
            # Look for images in other lazy loading attributes
            lazy_attrs = ['data-lazy-src', 'data-original', 'data-url']
            for attr in lazy_attrs:
                matches = re.findall(f'{attr}=\"([^\"]+)\"', html, re.IGNORECASE | re.DOTALL)
                for img_url in matches:
                    img_url = img_url.strip()
                    if any(img_url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif']):
                        skip_patterns = ['logo', 'banner', 'icon', 'button', 'watermark', 'placeholder', 'loading', 'avatar', 'thumb']
                        if not any(skip in img_url.lower() for skip in skip_patterns):
                            filename = img_url.split('/')[-1].split('.')[0]
                            if re.match(r'^\d+$', filename) or 'manga_' in img_url or filename in ['0-black']:
                                images.append(img_url)
            
            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(images))
            
            if len(unique_images) >= 1:
                return unique_images
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(4)
    
    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting StoneScape scraper")
    log("Mode: Full Downloader")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://stonescape.xyz", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

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
    for i, series_url in enumerate(all_series_urls):
        log(f"Processing: {series_url}")
        title = extract_series_title(session, series_url)
        if not title:
            error(f"No title found for {series_url}")
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

        # Determine padding width for chapter numbers
        if chapter_urls:
            chapter_nums = []
            for url in chapter_urls:
                match = re.search(r'ch-(\d+)', url)
                if match:
                    chapter_nums.append(int(match.group(1)))
            if chapter_nums:
                max_chapter = max(chapter_nums)
                padding_width = calculate_padding_width(max_chapter)
                log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Check for existing chapters
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

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

        # Process each chapter
        consecutive_skips = 0
        for ch_url in chapter_urls:
            # Extract chapter number
            num_match = re.search(r'ch-(\d+)', ch_url)
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
                error(f"Error extracting images for chapter {num}: {e}")
                continue

            if imgs is None:
                log(f"Skipping: Chapter {num} (no images)")
                consecutive_skips += 1
                if consecutive_skips >= 3:  # Stop after 3 consecutive non-existent chapters
                    log("Stopping due to 3 consecutive non-existent chapters")
                    break
                continue
            elif len(imgs) == 0:
                log(f"Skipping: Chapter {num} (no images)")
                consecutive_skips += 1
                if consecutive_skips >= 3:  # Stop after 3 consecutive non-existent chapters
                    log("Stopping due to 3 consecutive non-existent chapters")
                    break
                continue
            elif len(imgs) == 1:
                log(f"Skipping: Chapter {num} (no images)")
                consecutive_skips += 1
                if consecutive_skips >= 3:  # Stop after 3 consecutive non-existent chapters
                    log("Stopping due to 3 consecutive non-existent chapters")
                    break
                continue

            consecutive_skips = 0  # Reset on successful find

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {num} [{len(imgs)} images]")
                continue

            log(f"Downloading: Chapter {num} [{len(imgs)} images]")

            # Create chapter directory within series directory
            chapter_folder = series_directory / chapter_name
            chapter_folder.mkdir(parents=True, exist_ok=True)
            
            downloaded = 0
            total = len(imgs)

            for i, url in enumerate(imgs):
                idx = i + 1
                url = url.replace(' ', '%20')
                
                # Determine file extension
                ext = get_image_extension(url, 'jpg')
                
                file = chapter_folder / f"{idx:03d}.{ext}"

                try:
                    response = retry_request(url, session, timeout=120)
                    with open(file, 'wb') as f:
                        f.write(response.content)
                    print(f"  [{idx:03d}/{total:03d}] {url} Success", file=sys.stderr, flush=True)
                    downloaded += 1

                    # Convert to WebP if enabled
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

            # Add small delay between chapters to prevent rate limiting
            time.sleep(0.2)
            
            # Only create CBZ if more than 1 image was downloaded
            if downloaded > 1:
                if create_cbz(chapter_folder, chapter_name, series_directory):
                    shutil.rmtree(chapter_folder)
                else:
                    warn(f"CBZ creation failed for Chapter {num}, keeping folder")
            else:
                log(f"Skipping CBZ creation for Chapter {num} - only {downloaded} image(s) downloaded")
                # Remove temp folder
                shutil.rmtree(chapter_folder)

        # Add delay between series to prevent rate limiting
        time.sleep(0.5)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()