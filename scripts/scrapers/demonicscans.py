#!/usr/bin/env python3

import asyncio
import os
import re
import sys
import requests
import zipfile
from urllib.parse import urljoin, quote, urlparse, unquote
from pathlib import Path
from scraper_utils import log, success, warn, error, convert_to_webp, create_cbz, bypass_cloudflare, get_session, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'DemonicScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[DemonicScans]')
ALLOWED_DOMAINS = ['demoniclibs.com', 'mangareadon.org', 'readermc.org', 'mangafirst.org']
BASE_URL = 'https://demonicscans.org'

# Extract series URLs from the translations list page
def extract_series_urls(session, page_num):
    url = f"https://demonicscans.org/translationlist.php?page={page_num}"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for disabled "Next" button
    is_last_page = 'pointer-events: none' in html and 'Next' in html
    
    # Extract series URLs
    series_urls = re.findall(r'href="(/manga/[^"]+)"', html)
    return sorted(set(series_urls)), is_last_page

# Extract series title from series page
def extract_series_title(session, series_url):
    url = f"https://demonicscans.org{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        title = title_match.group(1).replace(' - Demonic Scans', '').strip()
        # Replace underscores with spaces (they may come from the HTML)
        title = title.replace('_', ' ')
        return title
    return None

# Extract chapter URLs for a given manga
def extract_chapter_urls(session, manga_url):
    full_url = f"https://demonicscans.org{manga_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\0', '')
    
    chapter_urls = re.findall(r'href="(/chaptered\.php\?manga=[^&]+&chapter=[^"]+)"', html)
    return sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter=(\d+)', x).group(1)))

# Extract image URLs for a given chapter
def extract_image_urls(session, chapter_url):
    full_url = f"https://demonicscans.org{chapter_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\0', '')
    
    image_urls = re.findall(r'https?://[^"]*\.(?:jpg|jpeg|png|webp|web)', html)
    filtered_urls = [url for url in image_urls if any(domain in url for domain in ALLOWED_DOMAINS) and 'thumbnails' not in url]
    return list(dict.fromkeys(filtered_urls))  # unique

def main():
    log("Starting Demonic Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://demonicscans.org...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://demonicscans.org", timeout=30)
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
        max_chapter = max(int(re.search(r'chapter=(\d+)', url).group(1)) for url in chapter_urls)
        padding_width = len(str(max_chapter))
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [DemonicScans].cbz"
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
            chapter_num = int(re.search(r'chapter=(\d+)', chapter_url).group(1))

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
                if not img_url or not any(domain in img_url for domain in ALLOWED_DOMAINS):
                    continue
                # Get extension
                parsed = urlparse(img_url)
                path = parsed.path
                ext = path.split('.')[-1].lower()
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