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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'ThunderScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[ThunderScans]')
ALLOWED_DOMAINS = ['en-thunderscans.com']
BASE_URL = 'https://en-thunderscans.com'

# Extract series URLs from comics page with pagination
def extract_series_urls(session, page_num):
    url = f"https://en-thunderscans.com/comics/?page={page_num}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page
    is_last_page = f'href="?page={page_num + 1}"' not in html or "No Post Found" in html
    
    # Extract series URLs
    series_urls = re.findall(r'href="https://en-thunderscans\.com/comics/[^"]*/"', html)
    series_urls = [url.replace('href="', '').rstrip('"') for url in series_urls]
    
    return sorted(set(series_urls)), is_last_page

# Extract series title from series page
def extract_series_title(session, series_url):
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        title = title_match.group(1).replace(' &#8211; Thunderscans EN', '').strip()
        return title
    
    return None

# Extract chapter links from series page
def extract_chapter_urls(session, series_url):
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter URLs
    chapter_urls = re.findall(r'href="https://en-thunderscans\.com/[^"]*chapter-[0-9]*/"', html)
    chapter_urls = [url.replace('href="', '').rstrip('"') for url in chapter_urls]
    
    return sorted(set(chapter_urls))

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\n', '')
    
    # Check if chapter is locked
    if "This chapter is locked" in html or "lock-container" in html:
        log("Chapter is locked, skipping")
        return []
    
    # Extract images JSON
    images_match = re.search(r'"images":\[([^\]]*)\]', html)
    if not images_match:
        log("No images JSON found")
        return []
    
    images_json = images_match.group(1)
    # Extract URLs
    image_urls = re.findall(r'https://[^"]*\.(?:webp|jpg|png)', images_json.replace('\\', ''))
    
    return image_urls

def main():
    log("Starting Thunder Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://en-thunderscans.com...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://en-thunderscans.com", timeout=30)
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
    while True:
        log(f"Fetching series list from page {page}...")
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
            warn(f"No title for {series_url}, skipping...")
            continue

        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")

        # Create series directory
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Extract chapter links
        try:
            chapter_urls = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        if not chapter_urls:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Extract chapter numbers for padding and skipping logic
        chapter_nums = []
        for url in chapter_urls:
            match = re.search(r'chapter-(\d+)', url)
            if match:
                chapter_nums.append(int(match.group(1)))

        if not chapter_nums:
            warn(f"No valid chapter numbers found for {title}, skipping...")
            continue

        max_chapter = max(chapter_nums)
        padding_width = len(str(max_chapter))
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [ThunderScans].cbz"
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

        for chapter_url in chapter_urls:
            chapter_number_match = re.search(r'chapter-(\d+)', chapter_url)
            if not chapter_number_match:
                continue
            chapter_number = int(chapter_number_match.group(1))

            # Skip if chapter already exists
            if chapter_number in existing_chapters:
                continue

            formatted_chapter_number = f"{chapter_number:0{padding_width}d}"
            chapter_name = f"{clean_title} Ch.{formatted_chapter_number} {DEFAULT_SUFFIX}"

            try:
                image_urls = extract_image_urls(session, chapter_url)
            except Exception as e:
                error(f"Error extracting images for chapter {chapter_url}: {e}")
                continue

            if not image_urls:
                log(f"Skipping: Chapter {chapter_number} (no images)")
                continue

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {chapter_number} [{len(image_urls)} images]")
                continue

            # Create chapter directory
            chapter_folder = series_directory / chapter_name
            chapter_folder.mkdir(parents=True, exist_ok=True)

            # Download images
            downloaded_count = 0
            for i, img_url in enumerate(image_urls, 0):
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
                    print(f"  [{i:03d}] {img_url} Success")
                    downloaded_count += 1
                    if CONVERT_TO_WEBP and ext != 'webp':
                        convert_to_webp(filename)
                except Exception as e:
                    print(f"  [{i:03d}] {img_url} Failed: {e}")

            log(f"Downloaded: Chapter {chapter_number} [{downloaded_count}/{len(image_urls)} images]")

            # Only create CBZ if more than 1 image was downloaded
            if downloaded_count > 1:
                if create_cbz(chapter_folder, chapter_name):
                    # Remove temp folder
                    import shutil
                    shutil.rmtree(chapter_folder)
                else:
                    warn(f"CBZ creation failed for Chapter {chapter_number}, keeping folder")
            else:
                log(f"Skipping CBZ creation for Chapter {chapter_number} - only {downloaded_count} image(s) downloaded")
                # Remove temp folder
                import shutil
                shutil.rmtree(chapter_folder)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")

if __name__ == "__main__":
    main()