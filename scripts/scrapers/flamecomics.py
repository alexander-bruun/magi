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
from scraper_utils import log, success, warn, error, convert_to_webp, create_cbz, get_session, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'FlameComics'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[FlameComics]')
ALLOWED_DOMAINS = ['cdn.flamecomics.xyz']
USER_AGENT = os.getenv('user_agent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36')

# Extract series URLs from browse page
def extract_series_urls(session, page_num):
    # Flame Comics doesn't have pagination, just one browse page
    if page_num > 1:
        return [], True
    
    url = "https://flamecomics.xyz/browse"
    log("Fetching series from browse page...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract series URLs (numeric IDs)
    series_urls = re.findall(r'href="/series/(\d+)"', html)
    full_urls = [f"https://flamecomics.xyz/series/{sid}" for sid in series_urls]
    return sorted(set(full_urls)), True  # is_last_page = True since no pagination

# Extract series title from series page
def extract_series_title(session, series_url):
    attempts = 3
    delay = 5
    
    for i in range(1, attempts + 1):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' - Flame Comics', '').strip()
                if title:
                    return title
        except Exception as e:
            if i < attempts:
                warn(f"Failed to extract title (attempt {i}/{attempts}), retrying in {delay}s... Error: {e}")
                import time
                time.sleep(delay)
    
    return None

# Extract chapter URLs from series page
def extract_chapter_urls(session, series_url):
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract __NEXT_DATA__ JSON
    json_match = re.search(r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL)
    if not json_match:
        error(f"Could not find __NEXT_DATA__ in {series_url}")
        return []
    
    try:
        data = json.loads(json_match.group(1))
        chapters = data.get('props', {}).get('pageProps', {}).get('chapters', [])
        
        # Sort by chapter number
        chapters.sort(key=lambda x: float(x.get('chapter', 0)))
        
        chapter_data = []
        for chapter in chapters:
            series_id = chapter.get('series_id')
            token = chapter.get('token')
            chapter_num = float(chapter.get('chapter', 0))
            if series_id and token:
                url = f"https://flamecomics.xyz/series/{series_id}/{token}"
                chapter_data.append((chapter_num, url))
        
        return chapter_data
    except (json.JSONDecodeError, KeyError) as e:
        error(f"Error parsing JSON from {series_url}: {e}")
        return []

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    attempts = 3
    delay = 5
    
    for i in range(1, attempts + 1):
        try:
            response = session.get(chapter_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            # Extract __NEXT_DATA__ JSON
            json_match = re.search(r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL)
            if not json_match:
                if i < attempts:
                    import time
                    time.sleep(delay)
                continue
            
            data = json.loads(json_match.group(1))
            chapter_data = data.get('props', {}).get('pageProps', {}).get('chapter', {})
            images = chapter_data.get('images', {})
            series_id = chapter_data.get('series_id')
            token = chapter_data.get('token')
            
            if not series_id or not token:
                if i < attempts:
                    import time
                    time.sleep(delay)
                continue
            
            urls = []
            for key, img_data in images.items():
                name = img_data.get('name', '')
                if 'commission' not in name:
                    url = f"https://cdn.flamecomics.xyz/uploads/images/series/{series_id}/{token}/{name}"
                    urls.append(url)
            
            if urls:
                return urls
        except Exception as e:
            if i < attempts:
                warn(f"Failed to extract images (attempt {i}/{attempts}), retrying in {delay}s... Error: {e}")
                import time
                time.sleep(delay)
    
    return []

def main():
    try:
        log("Starting Flame Comics scraper")
        log("Mode: Full Downloader")

        # Health check
        log("Performing health check on https://flamecomics.xyz...")
        try:
            session = requests.Session()
            session.headers.update({
                'User-Agent': USER_AGENT,
                'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
                'Accept-Language': 'en-US,en;q=0.5',
                'Accept-Encoding': 'gzip, deflate',
                'Connection': 'keep-alive',
                'Upgrade-Insecure-Requests': '1',
            })
            
            response = session.get("https://flamecomics.xyz", timeout=30)
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

        log(f"Found {len(all_series_urls)} series")

        total_series = len(all_series_urls)
        total_chapters = 0

        # Process each series
        for series_url in all_series_urls:
            if not series_url:
                continue

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
                chapter_data = extract_chapter_urls(session, series_url)
            except Exception as e:
                error(f"Error extracting chapters for {series_url}: {e}")
                continue

            if not chapter_data:
                warn(f"No chapters found for {title}, skipping...")
                continue

            # For Flame Comics, get actual chapter numbers
            chapter_numbers = [chapter_num for chapter_num, _ in chapter_data]
            max_chapter_number = max(chapter_numbers) if chapter_numbers else 0
            
            # Determine padding width based on the integer part of chapter numbers
            integer_parts = [int(chapter_num) for chapter_num in chapter_numbers]
            max_integer = max(integer_parts) if integer_parts else 0
            padding_width = len(str(max_integer))
            log(f"Found {len(chapter_data)} chapters (max: {max_chapter_number}, padding: {padding_width})")

            # Scan existing CBZ files to determine which chapters are already downloaded
            existing_chapters = set()
            for cbz_file in series_directory.glob("*.cbz"):
                # Extract chapter number from filename like "Title Ch.066.1 [FlameComics].cbz"
                match = re.search(r'Ch\.([\d.]+)', cbz_file.stem)
                if match:
                    try:
                        existing_chapters.add(float(match.group(1)))
                    except ValueError:
                        pass  # Skip invalid chapter numbers

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

            for chapter_num, chapter_url in chapter_data:
                # Skip if chapter already exists
                if chapter_num in existing_chapters:
                    continue

                total_chapters += 1

                # Format chapter number - use decimal if it's not a whole number
                if chapter_num == int(chapter_num):
                    formatted_chapter_number = f"{int(chapter_num):0{padding_width}d}"
                else:
                    formatted_chapter_number = f"{chapter_num:.1f}"

                chapter_name = f"{clean_title} Ch.{formatted_chapter_number} {DEFAULT_SUFFIX}"

                try:
                    image_urls = extract_image_urls(session, chapter_url)
                except Exception as e:
                    error(f"Error extracting images for {chapter_url}: {e}")
                    continue

                if not image_urls:
                    log(f"Skipping: Chapter {chapter_num} (no images)")
                    continue

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
    except Exception as e:
        error(f"Unexpected error in main(): {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()