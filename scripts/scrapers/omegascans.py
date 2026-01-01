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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'OmegaScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[OmegaScans]')
ALLOWED_DOMAINS = ['api.omegascans.org', 'media.omegascans.org']
API_BASE = os.getenv('api', 'https://api.omegascans.org')
BASE_URL = 'https://omegascans.org'

# Extract series data from API
def extract_series_urls(session, page_num):
    # Fetch all series in one go
    if page_num > 1:
        return [], True
    
    url = f"{API_BASE}/query/?page=1&perPage=99999999"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    data = response.json()
    
    series_data = []
    for series in data.get('data', []):
        series_type = series.get('series_type', '')
        if series_type != 'Novel':  # Skip novels
            series_data.append(series)
    
    return series_data, True  # is_last_page = True

# Extract series title from series data
def extract_series_title(session, series_data):
    return series_data.get('title', '')

# Extract chapter data from API for a series
def extract_chapter_urls(session, series_data):
    series_id = series_data.get('id')
    url = f"{API_BASE}/chapter/query?page=1&perPage=99999999&series_id={series_id}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    data = response.json()
    
    chapters = []
    for chapter in data.get('data', []):
        chapter_name_raw = chapter.get('chapter_name', '').strip()
        
        # Skip seasons and decimal chapters
        if 'season' in chapter_name_raw.lower() or '.' in chapter_name_raw:
            continue
        
        chapters.append(chapter)
    
    return chapters

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_data, series_data):
    series_slug = series_data.get('series_slug')
    chapter_slug = chapter_data.get('chapter_slug')
    chapter_url = f"https://omegascans.org/series/{series_slug}/{chapter_slug}"
    
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if premium
    if "This chapter is premium!" in html:
        return []
    
    # Extract image URLs from src attributes
    api_urls = re.findall(r'src="https://api\.omegascans\.org/uploads/series/[^"]+', html)
    media_urls = re.findall(r'src="https://media\.omegascans\.org/file/[^"]+', html)
    
    all_urls = api_urls + media_urls
    # Remove src=" prefix
    all_urls = [url.replace('src="', '') for url in all_urls]
    
    # Remove thumbnail if present
    thumbnail = series_data.get('thumbnail', '')
    if thumbnail:
        all_urls = [url for url in all_urls if url != thumbnail]
    
    return all_urls

def main():
    log("Starting Omega Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://omegascans.org...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://omegascans.org", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Fetch series data
    series_list, _ = extract_series_urls(session, 1)
    log(f"Found {len(series_list)} series")

    total_series = len(series_list)
    total_chapters = 0

    # Process each series
    for series_data in series_list:
        series_id = series_data.get('id')
        title = extract_series_title(session, series_data)
        clean_title = sanitize_title(title)

        log(f"Title: {clean_title}")

        # Create series directory
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Fetch chapters
        try:
            chapters = extract_chapter_urls(session, series_data)
        except Exception as e:
            error(f"Error fetching chapters for {title}: {e}")
            continue

        if not chapters:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Determine max chapter number for padding
        chapter_numbers = []
        for chapter in chapters:
            chapter_name_raw = chapter.get('chapter_name', '')
            nums = re.findall(r'\d+', chapter_name_raw)
            if nums:
                chapter_numbers.append(int(nums[0]))
        
        if not chapter_numbers:
            warn(f"No valid chapter numbers found for {title}, skipping...")
            continue

        max_chapter = max(chapter_numbers)
        padding_width = len(str(max_chapter))

        log(f"Found {len(chapters)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [OmegaScans].cbz"
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

        for chapter in chapters:
            chapter_name_raw = chapter.get('chapter_name', '').strip()
            nums = re.findall(r'\d+', chapter_name_raw)
            chapter_number = int(nums[0]) if nums else 0

            # Skip if chapter already exists
            if chapter_number in existing_chapters:
                continue
            
            total_chapters += 1

            formatted_chapter_number = f"{chapter_number:0{padding_width}d}"
            chapter_name = f"{clean_title} Ch.{formatted_chapter_number} {DEFAULT_SUFFIX}"

            try:
                image_urls = extract_image_urls(session, chapter, series_data)
            except Exception as e:
                error(f"Error extracting images for chapter {chapter_name_raw}: {e}")
                continue

            if not image_urls:
                log(f"Skipping: Chapter {chapter_number} (no images)")
                continue

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
                ext = path.split('.')[-1].lower() if '.' in path else 'jpg'
                if ext not in ['jpg', 'jpeg', 'png', 'webp']:
                    ext = 'jpg'  # default
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