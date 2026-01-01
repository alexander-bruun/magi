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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'AsuraScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[AsuraScans]')
ALLOWED_DOMAINS = ['gg.asuracomic.net']

# Extract series URLs from listing page
def extract_series_urls(session, page_num):
    url = f"https://asuracomic.net/series?page={page_num}"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for disabled "Next" button
    is_last_page = 'pointer-events: none' in html and 'Next' in html
    
    # Match series URLs with the 8-character hex suffix pattern (e.g., series/nano-machine-be19545a)
    series_urls = re.findall(r'href="series/[a-z0-9-]+-[a-f0-9]{8}"', html)
    # Remove href=" and add leading /
    series_urls = [url.replace('href="', '/').rstrip('"') for url in series_urls]
    return sorted(set(series_urls)), is_last_page

# Extract series title from series page
def extract_series_title(session, series_url):
    url = f"https://asuracomic.net{series_url}"
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(1, max_retries + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract title from <title> tag and remove " - Asura Scans" suffix
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' - Asura Scans', '').strip()
                return title
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
    full_url = f"https://asuracomic.net{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    series_slug = series_url.split('/series/')[-1]
    # Extract chapter links like series_slug/chapter/123
    chapter_patterns = re.findall(rf'{re.escape(series_slug)}/chapter/\d+', html)
    # Convert to full URLs
    chapter_urls = [f'/series/{pattern}' for pattern in chapter_patterns]
    # Sort by chapter number
    chapter_urls.sort(key=lambda x: int(re.search(r'/chapter/(\d+)', x).group(1)))
    return list(dict.fromkeys(chapter_urls))  # unique

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    full_url = f"https://asuracomic.net{chapter_url}"
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(1, max_retries + 1):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract image URLs from JSON data embedded in the page
            # The page contains escaped JSON with "order" and "url" fields
            # Try general pattern first
            pattern = r'\\"order\\":\d+,\\"url\\":\\"https://[^"\\]+\.(?:jpg|webp)\\"'
            matches = re.findall(pattern, html)
            
            if matches:
                # Extract URLs and sort by order
                urls_with_order = []
                for match in matches:
                    # Extract order and url
                    order_match = re.search(r'\\"order\\":(\d+)', match)
                    url_match = re.search(r'\\"url\\":\\"([^"\\]+)\\"', match)
                    if order_match and url_match:
                        order = int(order_match.group(1))
                        url = url_match.group(1)
                        urls_with_order.append((order, url))
                
                # Sort by order and extract URLs
                urls_with_order.sort(key=lambda x: x[0])
                image_urls = [url for _, url in urls_with_order]
                # Remove duplicates while preserving order
                seen = set()
                image_urls = [x for x in image_urls if not (x in seen or seen.add(x))]
                return image_urls
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
    log("Starting Asura Scans scraper")
    log("Mode: Full Downloader")

    # Create session
    session = get_session()

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
    for series_url in all_series_urls:
        log(f"Processing: {series_url}")

        title = extract_series_title(session, series_url)
        if not title:
            error(f"Could not extract title for {series_url}, skipping...")
            continue

        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")

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
        max_chapter = max(int(re.search(r'/chapter/(\d+)', url).group(1)) for url in chapter_urls)
        padding_width = len(str(max_chapter))
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [AsuraScans].cbz"
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
            chapter_num_match = re.search(r'/chapter/(\d+)', chapter_url)
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

            if DRY_RUN:
                log(f"Chapter {chapter_num} [{len(image_urls)} images]")
                continue

            # Create chapter directory
            chapter_folder = series_directory / chapter_name
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

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")

if __name__ == "__main__":
    main()