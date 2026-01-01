#!/usr/bin/env python3

import asyncio
import os
import re
import sys
import requests
import zipfile
from urllib.parse import urljoin, quote, urlparse
from pathlib import Path
from scraper_utils import log, success, warn, error, convert_to_webp, create_cbz, bypass_cloudflare, get_session, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'ZScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[ZScans]')
ALLOWED_DOMAINS = ['zscans.com']
BASE_URL = 'https://zscans.com'

# Extract series slugs from comics page
def extract_series_urls(session, page_num):
    # Z Scans doesn't have pagination, just one comics page
    if page_num > 1:
        return [], True
    
    url = "https://zscans.com/comics"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract series slugs from quoted strings, filter out common terms
    slugs = re.findall(r'"([a-z0-9-]+(?:-[a-z0-9-]+)*)"', html)
    # Filter out common HTML/CSS terms
    exclude_terms = {
        'css', 'js', 'png', 'jpg', 'webp', 'svg', 'ico', 'woff', 'ttf', 'eot', 'px', 'app', 'button', 'alert', 'all', 
        'canonical', 'charset', 'content', 'data', 'div', 'form', 'head', 'html', 'http', 'icon', 'img', 'input', 
        'link', 'meta', 'nav', 'none', 'page', 'path', 'rel', 'script', 'span', 'style', 'text', 'title', 'type', 
        'url', 'var', 'view', 'xml', 'lang', 'language', 'container', 'bookmark', 'horizontal', 'font-weight-bold', 
        'hooper-list', 'hooper-next', 'hooper-prev', 'hooper-track', 'action', 'comedy', 'drama', 'fantasy', 'horror', 
        'isekai', 'manga', 'manhua', 'manhwa', 'mystery', 'romance', 'supernatural', 'historical', 'completed', 
        'dropped', 'ongoing', 'hiatus', 'new', 'one-shot', 'martial-arts', 'reincarnation'
    }
    
    valid_slugs = []
    for slug in slugs:
        if slug not in exclude_terms and re.match(r'^[a-z][a-z0-9]*(-[a-z0-9]+)+$', slug):
            valid_slugs.append(slug)
    
    return sorted(set(valid_slugs)), True  # is_last_page = True

# Extract series title from series page
def extract_series_title(session, series_slug):
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(1, max_retries + 1):
        try:
            url = f"https://zscans.com/comics/{series_slug}"
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Try to extract title from JavaScript data first
            title_match = re.search(r'name:"([^"]*)"', html)
            if title_match:
                return title_match.group(1)
            
            # Fallback to page title
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' • Zero Scans', '').replace('Read ', '').replace(' with up to date chapters!', '')
                return title.strip()
        except Exception as e:
            if attempt < max_retries:
                warn(f"Failed to extract title (attempt {attempt}/{max_retries}), retrying in {retry_delay}s... Error: {e}")
                import time
                time.sleep(retry_delay)
    
    return None

# Extract chapter links from series page
def extract_chapter_urls(session, series_slug):
    url = f"https://zscans.com/comics/{series_slug}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter count and first chapter ID
    chapter_count_match = re.search(r'chapters_count:(\d+)', html)
    first_chapter_match = re.search(r'first_chapter:\[\{[^}]*?,id:(\d+)\}', html)
    
    if not chapter_count_match or not first_chapter_match:
        error(f"Could not find chapter information in {url}")
        return []
    
    chapter_count = int(chapter_count_match.group(1))
    first_chapter_id = int(first_chapter_match.group(1))
    
    # Generate chapter URLs assuming sequential IDs
    chapter_urls = []
    for i in range(chapter_count):
        chapter_id = first_chapter_id + i
        chapter_url = f"https://zscans.com/comics/{series_slug}/{chapter_id}"
        chapter_urls.append(chapter_url)
    
    return chapter_urls

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    max_retries = 3
    retry_delay = 5
    
    for attempt in range(1, max_retries + 1):
        try:
            response = session.get(chapter_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract image URLs from JavaScript data
            # Look for high_quality or good_quality arrays
            images_match = re.search(r'(high_quality|good_quality):\[(.*?)\]', html)
            if images_match:
                images_data = images_match.group(2)
                # Extract URLs and unescape \u002F to /
                urls = re.findall(r'"([^"]*)"', images_data)
                image_urls = []
                for url in urls:
                    url = url.replace('\\u002F', '/')
                    if url.startswith('https://'):
                        image_urls.append(url)
                return image_urls
        except Exception as e:
            if attempt < max_retries:
                warn(f"Failed to extract images (attempt {attempt}/{max_retries}), retrying in {retry_delay}s... Error: {e}")
                import time
                time.sleep(retry_delay)
    
    return []

def main():
    log("Starting Z Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://zscans.com...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://zscans.com", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Collect all series slugs
    all_series_slugs, _ = extract_series_urls(session, 1)
    log(f"Found {len(all_series_slugs)} series")

    total_series = len(all_series_slugs)
    total_chapters = 0

    # Process each series
    for series_slug in all_series_slugs:
        if not series_slug:
            continue

        log(f"Processing: {series_slug}")

        title = extract_series_title(session, series_slug)
        if not title:
            warn(f"Could not extract title for {series_slug}, skipping...")
            continue

        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")

        # Create series directory
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Extract chapter links
        try:
            chapter_urls = extract_chapter_urls(session, series_slug)
        except Exception as e:
            error(f"Error extracting chapters for {series_slug}: {e}")
            continue

        if not chapter_urls:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # For Z Scans, chapters are sequential
        max_chapter_number = len(chapter_urls)
        padding_width = len(str(max_chapter_number))
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter_number}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [ZScans].cbz"
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

        for chapter_index, chapter_url in enumerate(chapter_urls, 1):
            chapter_number = chapter_index

            # Skip if chapter already exists
            if chapter_number in existing_chapters:
                continue

            formatted_chapter_number = f"{chapter_number:0{padding_width}d}"
            chapter_name = f"{clean_title} Ch.{formatted_chapter_number} {DEFAULT_SUFFIX}"

            try:
                image_urls = extract_image_urls(session, chapter_url)
            except Exception as e:
                error(f"Error extracting images for {chapter_url}: {e}")
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
            for i, img_url in enumerate(image_urls, 1):
                if not img_url:
                    continue
                # Get extension
                parsed = urlparse(img_url)
                path = parsed.path
                ext = path.split('.')[-1].lower() if '.' in path else 'webp'
                if ext not in ['jpg', 'jpeg', 'png', 'webp']:
                    ext = 'webp'  # default
                filename = chapter_folder / f"{i:03d}.{ext}"
                try:
                    response = session.get(img_url, timeout=30)
                    response.raise_for_status()
                    with open(filename, 'wb') as f:
                        f.write(response.content)
                    print(f"  [{i:03d}/{len(image_urls):03d}] {img_url} Success")
                    downloaded_count += 1
                    if CONVERT_TO_WEBP and ext != 'webp':
                        convert_to_webp(filename)
                except Exception as e:
                    print(f"  [{i:03d}/{len(image_urls):03d}] {img_url} Failed: {e}")

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