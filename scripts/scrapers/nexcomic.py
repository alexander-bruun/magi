#!/usr/bin/env python3

import asyncio
import os
import re
import sys
import requests
import zipfile
import json
from urllib.parse import urljoin, quote, urlparse
from pathlib import Path
from scraper_utils import log, success, warn, error, convert_to_webp, create_cbz, bypass_cloudflare, get_session, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'NexComic'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[NexComic]')
ALLOWED_DOMAINS = ['nexcomic.com', 'storage.nexcomic.com']  # Adjust if they use external storage
USER_AGENT = os.getenv('user_agent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36')
BASE_URL = 'https://nexcomic.com'

def retry_request(url, session, max_retries=3, timeout=60):
    """Make a request with retry logic and exponential backoff for rate limiting."""
    for attempt in range(max_retries):
        try:
            response = session.get(url, timeout=timeout)
            if response.status_code == 429:
                wait_time = 2 ** attempt  # Exponential backoff: 1s, 2s, 4s
                warn(f"Rate limited (429). Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})")
                import time
                time.sleep(wait_time)
                continue
            response.raise_for_status()
            return response
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt
                warn(f"Request failed: {e}. Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})")
                import time
                time.sleep(wait_time)
            else:
                raise e

# Extract series URLs from manga listing page
def extract_series_urls(session, page_num):
    if page_num > 1:
        # For now, assume single page - can be extended for pagination
        return [], True
    
    log("Fetching series from manga listing page...")
    response = retry_request("https://nexcomic.com/manga/", session)
    html = response.text.replace('\n', '')
    
    # Extract series URLs from the listing
    series_urls = []
    # Look for links to /manga/{slug}/ - more flexible regex
    all_manga_links = re.findall(r'/manga/[^"\s\'<>]*', html)
    
    for link in all_manga_links:
        # Filter out non-series links
        if any(skip in link for skip in ['/feed', '/page/', '/#', '/list-mode']):
            continue
        # Must be a series link (should have a slug after /manga/)
        if link.count('/') >= 3 and not link.endswith('/manga/'):
            if link not in series_urls:
                series_urls.append(link)
    
    return series_urls, True  # is_last_page = True for now

# Extract series title from series page
def extract_series_title(session, series_url):
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
            title = re.sub(r'\s*\|\s*NexComic.*$', '', title, re.IGNORECASE)
            return title
    
    return None

# Extract chapter links from series page
def extract_chapter_urls(session, series_url):
    full_url = urljoin(BASE_URL, series_url)
    
    response = retry_request(full_url, session)
    html = response.text.replace('\n', '')
    
    # Extract chapter URLs - look for links to chapter pages
    chapter_urls = []
    
    # Pattern for chapter links
    chapter_patterns = [
        r'href="([^"]*chapter-[^"]*)"',
        r'href="(/[^"]*chapter[^"]*)"',
    ]
    
    for pattern in chapter_patterns:
        matches = re.findall(pattern, html)
        for match in matches:
            if match not in chapter_urls:
                chapter_urls.append(match)
    
    # Sort chapters by number
    def extract_chapter_num(url):
        match = re.search(r'chapter-(\d+)', url)
        return int(match.group(1)) if match else 0
    
    chapter_urls.sort(key=extract_chapter_num)
    
    return chapter_urls

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    full_url = urljoin(BASE_URL, chapter_url)
    
    for attempt in range(3):
        try:
            response = retry_request(full_url, session)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text
            
            # Check if chapter is locked or unavailable
            if "locked" in html.lower() or "not available" in html.lower():
                return None  # Locked/unavailable
            
            # Extract image URLs - try various patterns
            images = []
            
            # Look for img tags with src
            img_matches = re.findall(r'<img[^>]*src="([^"]*)"', html)
            for img_url in img_matches:
                if any(img_url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif']):
                    # Filter out UI elements, logos, watermarks, etc.
                    skip_patterns = ['logo', 'banner', 'icon', 'button', 'watermark', 'placeholder', 'loading', 'avatar', 'thumb']
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        # Only include images that look like chapter pages (numbered files)
                        # Chapter images typically have patterns like 00.jpg, 01.png, 001.webp, etc.
                        filename = img_url.split('/')[-1].split('.')[0]  # Get filename without extension
                        if re.match(r'^\d+$', filename):  # Only digits
                            images.append(img_url)
            
            # Also look for data-src or similar lazy loading attributes
            data_src_matches = re.findall(r'data-src="([^"]*)"', html)
            for img_url in data_src_matches:
                if any(img_url.endswith(ext) for ext in ['.webp', '.jpg', '.png', '.jpeg', '.avif']):
                    skip_patterns = ['logo', 'banner', 'icon', 'button', 'watermark', 'placeholder', 'loading', 'avatar', 'thumb']
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        filename = img_url.split('/')[-1].split('.')[0]
                        if re.match(r'^\d+$', filename):
                            images.append(img_url)
            
            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(images))
            
            if len(unique_images) >= 1:
                return unique_images
        except Exception as e:
            if attempt < 2:
                import time
                time.sleep(4)
    
    return []

def main():
    log("Starting NexComic scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://nexcomic.com...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://nexcomic.com", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Collect all series URLs
    all_series_urls = []
    page = 1
    page_series, is_last_page = extract_series_urls(session, page)
    all_series_urls.extend(page_series)

    total_series = len(all_series_urls)
    total_chapters = 0

    log(f"Found {total_series} series to process")

    # Process each series
    for series_url in all_series_urls:
        title = extract_series_title(session, series_url)
        if not title:
            error(f"No title found for {series_url}")
            continue

        clean_title = sanitize_title(title)
        log(f"Title: {clean_title}")

        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Extract chapter URLs
        try:
            chapter_urls = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        log(f"Found {len(chapter_urls)} chapters")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.01 [NexComic].cbz"
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

        # Process each chapter
        for ch_url in chapter_urls:
            # Extract chapter number
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
                log(f"Skipping: Chapter {num} (locked/unavailable)")
                continue
            elif len(imgs) == 0:
                log(f"Skipping: Chapter {num} (not found)")
                continue
            elif len(imgs) == 1:
                log(f"Skipping: Chapter {num} (only 1 image)")
                continue

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {num} [{len(imgs)} images]")
                continue

            # Create chapter directory within series directory
            chapter_folder = series_directory / name
            chapter_folder.mkdir(parents=True, exist_ok=True)
            
            downloaded = 0
            total = len(imgs)

            for i, url in enumerate(imgs):
                idx = i + 1
                url = url.replace(' ', '%20')
                
                # Determine file extension
                parsed = urlparse(url)
                ext = '.' + parsed.path.split('.')[-1].lower()
                if ext not in ['.webp', '.jpg', '.png', '.jpeg', '.avif']:
                    ext = '.jpg'  # Default fallback
                
                file = chapter_folder / f"{idx:03d}{ext}"

                try:
                    response = retry_request(url, session, timeout=120)
                    with open(file, 'wb') as f:
                        f.write(response.content)
                    print(f"  [{idx:03d}/{total:03d}] {url} Success")
                    downloaded += 1

                    # Convert to WebP if enabled
                    if CONVERT_TO_WEBP and ext != '.webp':
                        convert_to_webp(file)
                except Exception as e:
                    print(f"  [{idx:03d}/{total:03d}] {url} Failed: {e}")
                    # Clean up and break
                    import shutil
                    shutil.rmtree(chapter_folder)
                    break

            if downloaded != total:
                warn("Incomplete → skipped")
                continue

            log(f"Downloaded: Chapter {num} [{downloaded}/{len(imgs)} images]")
            
            # Add small delay between chapters to prevent rate limiting
            import time
            time.sleep(0.2)
            
            if create_cbz(chapter_folder, name, series_directory):
                import shutil
                shutil.rmtree(chapter_folder)
            else:
                warn(f"CBZ creation failed for Chapter {num}, keeping folder")

        # Add delay between series to prevent rate limiting
        import time
        time.sleep(0.5)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")

if __name__ == "__main__":
    main()