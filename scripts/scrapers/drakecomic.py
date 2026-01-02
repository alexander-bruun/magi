#!/usr/bin/env python3
"""
DrakeComic scraper for MAGI.

Downloads manga/manhwa/manhua from drakecomic.org.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import sys
import time
from pathlib import Path

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    convert_to_webp,
    create_cbz,
    check_duplicate_series,
    get_priority_config,
    error,
    get_existing_chapters,
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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'DrakeComic'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[DrakeComic]')
ALLOWED_DOMAINS = ['drakecomic.org']
USER_AGENT = os.getenv('user_agent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36')
BASE_URL = 'https://drakecomic.org'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('drakecomic')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from pagination.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    url = f"{BASE_URL}/manga/?page={page_num}&status=&type=&order="
    
    try:
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text
        
        # Extract series URLs from the page
        # Look for links to individual manga pages
        series_urls = re.findall(r'href="(https://drakecomic\.org/manga/[^"]*/)"', html)
        
        # Remove duplicates
        series_urls = list(set(series_urls))
        
        # Check if there's a next page
        has_next_page = 'page=' + str(page_num + 1) in html or f'?page={page_num + 1}' in html
        
        return series_urls, not has_next_page  # is_last_page
    
    except Exception as e:
        error(f"Error extracting series from page {page_num}: {e}")
        return [], True


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text
        
        # Look for the title in various possible locations
        title_match = re.search(r'<h1[^>]*>([^<]+)</h1>', html)
        if title_match:
            return title_match.group(1).strip()
        
        # Try other patterns
        title_match = re.search(r'<title>([^|]+)', html)
        if title_match:
            return title_match.group(1).strip()
            
        return None
        
    except Exception as e:
        error(f"Error extracting title from {series_url}: {e}")
        return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter links from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        list: Chapter URLs sorted by chapter number
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text.replace('\n', '')
        
        # Extract chapter URLs - looking for links to chapter pages
        chapter_urls = re.findall(r'href="(https://drakecomic\.org/[^"]*chapter-\d+[^"]*)"', html)
        
        # Remove duplicates and sort
        chapter_urls = list(set(chapter_urls))
        
        # Sort by chapter number
        def chapter_sort_key(url):
            match = re.search(r'chapter-(\d+)', url)
            return int(match.group(1)) if match else 0
        
        chapter_urls.sort(key=chapter_sort_key)
        
        return chapter_urls
        
    except Exception as e:
        error(f"Error extracting chapters from {series_url}: {e}")
        return []


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs in reading order, None if locked, empty list if no images
    """
    full_url = chapter_url
    
    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(full_url, timeout=30)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text
            
            # Check if chapter is locked or premium - be more specific
            locked_indicators = [
                "this chapter is locked",
                "chapter locked", 
                "premium content only",
                "login required to view",
                "please login to read",
                "members only content"
            ]
            is_locked = any(indicator in html.lower() for indicator in locked_indicators)
            
            # Extract image URLs - try multiple patterns
            images = []
            
            # Pattern 1: src attributes
            src_images = re.findall(r'src="(https://drakecomic\.org/wp-content/uploads/[^"]*\.(?:webp|jpg|png|jpeg|avif))"', html)
            images.extend(src_images)
            
            # Pattern 2: direct URLs in HTML
            direct_images = re.findall(r'https://drakecomic\.org/wp-content/uploads/[^\s"<>\']*\.(?:webp|jpg|png|jpeg|avif)', html)
            images.extend(direct_images)
            
            # Pattern 3: data-src attributes (lazy loading)
            data_src_images = re.findall(r'data-src="(https://drakecomic\.org/wp-content/uploads/[^"]*\.(?:webp|jpg|png|jpeg|avif))"', html)
            images.extend(data_src_images)
            
            # Pattern 4: data-url attributes
            data_url_images = re.findall(r'data-url="(https://drakecomic\.org/wp-content/uploads/[^"]*\.(?:webp|jpg|png|jpeg|avif))"', html)
            images.extend(data_url_images)
            
            # Pattern 5: JavaScript sources array (most important for this site)
            # Look for the images array in ts_reader.run() JavaScript
            js_match = re.search(r'"images":\s*\[([^\]]+)\]', html, re.DOTALL)
            if js_match:
                images_block = js_match.group(1)
                # Extract individual image URLs from the images array (handles escaped URLs)
                js_images = re.findall(r'"(https:\\\/\\\/drakecomic\.org\\\/wp-content\\\/uploads[^"]*\.(?:webp|jpg|png|jpeg|avif))"', images_block)
                # Unescape the URLs
                unescaped_images = [url.replace('\\/', '/') for url in js_images]
                images.extend(unescaped_images)
            
            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(images))
            
            # Filter out small images, logos, etc.
            filtered_images = []
            for url in unique_images:
                # Skip if it's clearly a UI element or thumbnail
                skip_keywords = ['logo', 'icon', 'button', 'avatar', 'thumbnail', 'thumb', 'cropped', 'small', 'favicon']
                if any(skip_word in url.lower() for skip_word in skip_keywords):
                    continue
                
                # Skip images with dimension specifications in filename (e.g., -227x300, -32x32)
                filename = url.split('/')[-1]
                if re.search(r'-\d+x\d+', filename):
                    continue
                
                # Must be from wp-content/uploads
                if '/wp-content/uploads/' in url:
                    filtered_images.append(url)
            
            # If we found images, the chapter is not locked
            if filtered_images:
                is_locked = False
            
            if is_locked:
                log(f"Chapter appears locked (no images found)")
                return None  # Locked
            
            # If we found images, the chapter is not locked
            if filtered_images:
                is_locked = False
            
            if is_locked:
                log(f"Chapter appears locked (no valid images found)")
                return None  # Locked
            
            if len(filtered_images) >= 1:
                return filtered_images
            else:
                log(f"No valid comic images found in chapter {chapter_url}")
                return []
                
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(RETRY_DELAY)
    
    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting DrakeComic scraper")
    log("Mode: Full Downloader")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        # Try without Cloudflare bypass first
        session = requests.Session()
        session.headers.update({
            'User-Agent': USER_AGENT,
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'en-US,en;q=0.5',
            'Accept-Encoding': 'gzip, deflate',
            'Connection': 'keep-alive',
            'Upgrade-Insecure-Requests': '1',
        })
        
        response = session.get(BASE_URL, timeout=30)
        if response.status_code != 200:
            log("Direct access failed, trying Cloudflare bypass...")
            cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
            if not cookies:
                return
            session = get_session(cookies, headers)
            response = session.get(BASE_URL, timeout=30)
            if response.status_code != 200:
                error(f"Health check failed. Returned {response.status_code}")
                return
        else:
            log("Direct access successful")
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
        log(f"Extracting series from page {page}...")
        page_series, is_last_page = extract_series_urls(session, page)
        all_series_urls.extend(page_series)
        log(f"Found {len(page_series)} series on page {page}")
        
        if is_last_page or page >= 50:  # Safety limit
            break
        page += 1

    total_series = len(all_series_urls)
    total_chapters = 0

    log(f"Total series found: {total_series}")

    # Process each series
    for series_url in all_series_urls:
        title = extract_series_title(session, series_url)
        if not title:
            error(f"No title found for {series_url} → skip")
            continue

        clean_title = sanitize_title(title)
        if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS, PRIORITY):
            log(f"Skipping {clean_title} due to duplicate in higher priority provider")
            continue
        log(f"Processing: {clean_title}")

        # Extract chapter URLs
        try:
            chapter_urls = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        if not chapter_urls:
            warn(f"No chapters found for {clean_title}, skipping...")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        log(f"Found {len(chapter_urls)} chapters")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for ch_url in chapter_urls:
            # Extract chapter number from URL
            num_match = re.search(r'chapter-(\d+)', ch_url)
            if not num_match:
                continue
            num = int(num_match.group(1))

            # Skip if chapter already exists
            if num in existing_chapters:
                continue

            padded = f"{num:02d}"
            name = f"{clean_title} Ch.{padded} {DEFAULT_SUFFIX}"

            try:
                imgs = extract_image_urls(session, ch_url)
            except Exception as e:
                error(f"Error extracting images for chapter {num}: {e}")
                continue

            if imgs is None:
                log(f"Skipping: Chapter {num} (locked/premium)")
                continue
            elif len(imgs) == 0:
                log(f"Skipping: Chapter {num} (no images found)")
                continue
            elif len(imgs) == 1:
                log(f"Skipping: Chapter {num} (only 1 image)")
                continue

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {num} [{len(imgs)} images]")
                continue

            log(f"Downloading: Chapter {num} [{len(imgs)} images]")

            # Create chapter directory within series directory
            chapter_folder = series_directory / name
            chapter_folder.mkdir(parents=True, exist_ok=True)
            
            downloaded = 0
            total = len(imgs)

            for i, url in enumerate(imgs):
                idx = i + 1
                url = url.replace(' ', '%20')
                ext = '.' + url.split('.')[-1].split('?')[0]  # Handle query parameters
                file = chapter_folder / f"{idx:03d}{ext}"

                try:
                    response = session.get(url, timeout=120)
                    response.raise_for_status()
                    with open(file, 'wb') as f:
                        f.write(response.content)
                    print(f"  [{idx:03d}/{total:03d}] {url} Success", file=sys.stderr, flush=True)
                    downloaded += 1

                    # Convert to WebP if enabled
                    ext = file.suffix.lower()
                    if CONVERT_TO_WEBP and ext != '.webp':
                        convert_to_webp(file)
                except Exception as e:
                    print(f"  [{idx:03d}/{total:03d}] {url} Failed: {e}", file=sys.stderr, flush=True)
                    # Clean up and break
                    shutil.rmtree(chapter_folder)
                    break

            if downloaded != total:
                warn(f"Incomplete download for Chapter {num} → skipped")
                continue

            if create_cbz(chapter_folder, name, series_directory):
                shutil.rmtree(chapter_folder)
            else:
                warn(f"CBZ creation failed for Chapter {num}, keeping folder")

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()