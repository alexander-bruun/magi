#!/usr/bin/env python3

import os
import re
import sys
import requests
import zipfile
from urllib.parse import urljoin, quote, urlparse
from pathlib import Path

# Import common utilities
from scraper_utils import log, success, warn, error, get_session, convert_to_webp, create_cbz, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'MangaKatana'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[MangaKatana]')
ALLOWED_DOMAINS = ['mangakatana.com', 'i1.mangakatana.com', 'i2.mangakatana.com', 'i3.mangakatana.com', 'i4.mangakatana.com', 'i5.mangakatana.com', 'i6.mangakatana.com', 'i7.mangakatana.com']
BASE_URL = 'https://mangakatana.com'

# Extract series URLs from listing page
def extract_series_urls(session, page_num):
    if page_num == 1:
        url = f"{BASE_URL}/latest"
    else:
        url = f"{BASE_URL}/latest/page/{page_num}"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for pagination
    # MangaKatana shows "next" link if there are more pages
    is_last_page = 'next page-numbers' not in html
    
    # Extract series URLs - look for full manga URLs
    series_urls = re.findall(r'href="(https://mangakatana\.com/manga/[^"]+\.\d+)"', html)
    # Convert to relative URLs for consistency
    series_urls = [url.replace('https://mangakatana.com', '') for url in series_urls]
    # Filter out chapter URLs (those containing /c)
    series_urls = [url for url in series_urls if '/c' not in url]
    return sorted(set(series_urls)), is_last_page

# Extract series title from series page
def extract_series_title(session, series_url):
    url = f"{BASE_URL}{series_url}"
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(1, max_retries + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract title from various possible locations
            # Try title tag first
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' - MangaKatana', '').strip()
                return title
            
            # Try h1 tag
            h1_match = re.search(r'<h1[^>]*>([^<]+)</h1>', html)
            if h1_match:
                return h1_match.group(1).strip()
                
        except Exception as e:
            if attempt < max_retries:
                warn(f"Failed to extract title (attempt {attempt}/{max_retries}), retrying in {retry_delay}s... Error: {e}")
                import time
                time.sleep(retry_delay)
            else:
                error(f"Failed to extract title after {max_retries} attempts: {e}")
                return None
    
    return None

# Extract chapter URLs from series page
def extract_chapter_urls(session, series_url):
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter links - they follow pattern https://mangakatana.com/manga/series.id/c123
    chapter_urls = re.findall(r'href="(https://mangakatana\.com/manga/[^"]+/c[\d.]+[^"]*)"', html)
    # Convert to relative URLs
    chapter_urls = [url.replace('https://mangakatana.com', '') for url in chapter_urls]
    # Sort by chapter number (handle decimals)
    def chapter_key(url):
        match = re.search(r'/c([\d.]+)', url)
        if match:
            return float(match.group(1))
        return 0
    chapter_urls.sort(key=chapter_key)
    return list(dict.fromkeys(chapter_urls))  # unique

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    full_url = f"{BASE_URL}{chapter_url}"
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(1, max_retries + 1):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract image URLs from the JavaScript array thzq
            # Look for var thzq=['url1','url2',...];
            thzq_match = re.search(r'var thzq=\[([^\]]+)\];', html)
            if thzq_match:
                # Extract URLs from the array
                array_content = thzq_match.group(1)
                image_urls = re.findall(r"'(https://[^']+)'", array_content)
            else:
                # Fallback to looking for img tags
                image_urls = re.findall(r'<img[^>]*src="([^"]*mangakatana\.com[^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
            
            # Filter for allowed domains and remove thumbnails
            filtered_urls = []
            for url in image_urls:
                if any(domain in url for domain in ALLOWED_DOMAINS) and 'thumbnail' not in url.lower():
                    filtered_urls.append(url)
            
            # Sort by filename number if present
            def sort_key(url):
                match = re.search(r'/(\d+)\.(jpg|jpeg|png|webp)', url)
                if match:
                    return int(match.group(1))
                return 0
            
            filtered_urls.sort(key=sort_key)
            return list(dict.fromkeys(filtered_urls))  # unique
            
        except Exception as e:
            if attempt < max_retries:
                warn(f"Failed to extract images (attempt {attempt}/{max_retries}), retrying in {retry_delay}s... Error: {e}")
                import time
                time.sleep(retry_delay)
            else:
                error(f"Failed to extract images after {max_retries} attempts: {e}")
                return []
    
    return []

def main():
    log("Starting MangaKatana scraper")
    log("Mode: Full Downloader")

    # Create session with custom headers
    session = get_session()
    session.headers.update({
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.5',
        'Accept-Encoding': 'gzip, deflate',
        'Connection': 'keep-alive',
        'Upgrade-Insecure-Requests': '1',
        'Sec-Fetch-Dest': 'document',
        'Sec-Fetch-Mode': 'navigate',
        'Sec-Fetch-Site': 'none',
        'Cache-Control': 'max-age=0',
    })

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Process series page by page
    page = 1
    total_series = 0
    total_chapters = 0
    while page <= 5:  # Only process first 5 pages for testing
        try:
            page_series, is_last_page = extract_series_urls(session, page)
            if not page_series:
                log(f"No series found on page {page}, stopping.")
                break
            log(f"Found {len(page_series)} series on page {page}")
            
            # Process each series on this page
            for series_url in page_series:
                total_series += 1
                log(f"Processing series {total_series}: {series_url}")

                title = extract_series_title(session, series_url)
                if not title:
                    error(f"Could not extract title for {series_url}, skipping...")
                    continue

                clean_title = sanitize_title(title)
                log(f"Title: {clean_title}")

                # Add delay after extracting title
                import time
                # time.sleep(1)

                # Create series directory
                series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
                series_directory.mkdir(parents=True, exist_ok=True)

                # Extract chapter URLs
                try:
                    chapter_urls = extract_chapter_urls(session, series_url)
                except Exception as e:
                    error(f"Error extracting chapters for {series_url}: {e}")
                    continue

                if not chapter_urls:
                    warn(f"No chapters found for {title}, skipping...")
                    continue

                # Determine padding width
                max_chapter = max(int(re.search(r'/c(\d+)', url).group(1)) for url in chapter_urls)
                padding_width = len(str(max_chapter))
                log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

                # Add delay after extracting chapters
                # time.sleep(1)

                # Scan existing CBZ files to determine which chapters are already downloaded
                existing_chapters = set()
                for cbz_file in series_directory.glob("*.cbz"):
                    # Extract chapter number from filename like "Title Ch.001 [MangaKatana].cbz"
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
                for chapter_url in chapter_urls:
                    chapter_num_match = re.search(r'/c(\d+)', chapter_url)
                    if not chapter_num_match:
                        continue
                    chapter_num = int(chapter_num_match.group(1))

                    # Skip if chapter already exists
                    if chapter_num in existing_chapters:
                        continue

                    total_chapters += 1

                    formatted_chapter_number = f"{chapter_num:0{padding_width}d}"

                    chapter_name = f"{clean_title} Ch.{formatted_chapter_number} {DEFAULT_SUFFIX}"

                    try:
                        image_urls = extract_image_urls(session, chapter_url)
                    except Exception as e:
                        error(f"Error extracting images for {chapter_url}: {e}")
                        continue

                    if not image_urls:
                        log(f"Skipping: Chapter {chapter_num} (no images)")
                        consecutive_skips += 1
                        if consecutive_skips >= 3:  # Stop after 3 consecutive non-existent chapters
                            log("Stopping due to 3 consecutive non-existent chapters")
                            break
                        continue

                    consecutive_skips = 0  # Reset on successful find

                    # Add delay after extracting images
                    # time.sleep(0.5)

                    if DRY_RUN:
                        log(f"Chapter {chapter_num} [{len(image_urls)} images]")
                        continue

                    # Create chapter directory - use chapter name for consistency with other scrapers
                    chapter_folder = series_directory / chapter_name
                    chapter_folder.mkdir(parents=True, exist_ok=True)
                    
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

                    # Add delay between chapters
                    # time.sleep(0.5)

                # Add delay between series
                # time.sleep(2)

            if is_last_page:
                log(f"Reached last page (page {page}).")
                break
            page += 1
            # Add delay between pages
            # time.sleep(2)
        except Exception as e:
            error(f"Error processing page {page}: {e}")
            break

if __name__ == "__main__":
    main()