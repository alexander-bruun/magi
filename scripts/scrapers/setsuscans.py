#!/usr/bin/env python3

import asyncio
import json
import os
import re
import sys
import requests
from urllib.parse import urljoin, quote, urlparse
from pathlib import Path
from scraper_utils import log, success, warn, error, convert_to_webp, create_cbz, bypass_cloudflare, get_session, sanitize_title

# Configuration
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'SetsuScans'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[SetsuScans]')
ALLOWED_DOMAINS = ['setsuscans.com']
BASE_URL = 'https://setsuscans.com'

# Extract series URLs using AJAX pagination
def extract_series_urls(session, page_num):
    url = "https://setsuscans.com/wp-admin/admin-ajax.php"
    data = {
        'action': 'madara_load_more',
        'page': page_num,
        'template': 'madara-core/content/content-archive',
        'vars[paged]': page_num,
        'vars[orderby]': 'date',
        'vars[template]': 'archive',
        'vars[sidebar]': 'right',
        'vars[post_type]': 'wp-manga',
        'vars[post_status]': 'publish',
        'vars[meta_query][relation]': 'AND',
        'vars[manga_archives_item_layout]': 'big_thumbnail'
    }

    headers = {
        'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
        'X-Requested-With': 'XMLHttpRequest',
        'Referer': 'https://setsuscans.com/?m_orderby=new-manga'
    }

    log(f"Fetching series list from page {page_num}...")

    try:
        response = session.post(url, data=data, headers=headers, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if this is the last page (no more content)
        is_last_page = len(html.strip()) < 100  # Empty or minimal response

        # Extract series URLs from the HTML response
        series_urls = re.findall(r'href="https://setsuscans\.com(/manga/[^/]+/)"', html)
        # Filter out chapter URLs and other non-series URLs
        series_urls = [url for url in series_urls if 'chapter' not in url and 'feed' not in url and 'genre' not in url]

        return sorted(set(series_urls)), is_last_page

    except Exception as e:
        error(f"Error fetching page {page_num}: {e}")
        return [], True

# Extract series title from series page
def extract_series_title(session, series_url):
    url = f"https://setsuscans.com{series_url}"
    try:
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Try multiple selectors for title
        title_match = re.search(r'<h1[^>]*class="[^"]*entry-title[^"]*"[^>]*>([^<]+)</h1>', html)
        if not title_match:
            title_match = re.search(r'<title>([^<]+)', html)

        if title_match:
            title = title_match.group(1).replace(' – Setsu Scans', '').replace(' | Setsu Scans', '').strip()
            return title
    except Exception as e:
        error(f"Error extracting title from {url}: {e}")

    return None

# Extract chapter URLs using AJAX endpoint
def extract_chapter_urls(session, manga_url):
    # Extract manga slug from URL
    slug_match = re.search(r'/manga/([^/]+)/', manga_url)
    if not slug_match:
        return []

    slug = slug_match.group(1)
    url = f"https://setsuscans.com/manga/{slug}/ajax/chapters/"

    headers = {
        'X-Requested-With': 'XMLHttpRequest',
        'Referer': f"https://setsuscans.com/manga/{slug}/"
    }

    try:
        response = session.post(url, headers=headers, timeout=30)
        response.raise_for_status()
        html = response.text

        # Extract chapter URLs from the AJAX response
        chapter_urls = re.findall(r'href="https://setsuscans\.com(/manga/[^/]+/chapter-[^/]+/)"', html)
        # Remove duplicates and sort by chapter number
        unique_urls = sorted(set(chapter_urls), key=lambda x: extract_chapter_number(x))
        return unique_urls

    except Exception as e:
        error(f"Error extracting chapters for {manga_url}: {e}")
        return []

def extract_chapter_number(url):
    """Extract chapter number for sorting"""
    match = re.search(r'chapter-(\d+)', url)
    if match:
        return int(match.group(1))
    return 0

# Extract image URLs for a given chapter
def extract_image_urls(session, chapter_url):
    full_url = f"https://setsuscans.com{chapter_url}"
    try:
        response = session.get(full_url, timeout=30)
        response.raise_for_status()
        html = response.text.replace('\0', '')

        # Look for img src attributes that contain manga images
        image_urls = re.findall(r'<img[^>]*src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)

        # Clean URLs and filter for WP-manga images
        cleaned_urls = []
        for url in image_urls:
            url = url.strip()
            if 'WP-manga' in url and 'thumbnails' not in url and 'data/manga' in url:
                # Remove any leading/trailing whitespace or encoded characters
                url = re.sub(r'^[^h]*', '', url)  # Remove anything before 'http'
                if url.startswith('http'):
                    cleaned_urls.append(url)

        return list(dict.fromkeys(cleaned_urls))  # unique

    except Exception as e:
        error(f"Error extracting images from {chapter_url}: {e}")
        return []

def main():
    log("Starting Setsu Scans scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://setsuscans.com...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://setsuscans.com", timeout=30)
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
            if page > 50:  # Safety limit
                log("Reached safety limit of 50 pages, stopping.")
                break
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
        chapter_numbers = [extract_chapter_number(url) for url in chapter_urls]
        if chapter_numbers:
            max_chapter = max(chapter_numbers)
            padding_width = len(str(max_chapter))
        else:
            padding_width = 3
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = set()
        for cbz_file in series_directory.glob("*.cbz"):
            # Extract chapter number from filename like "Title Ch.001 [SetsuScans].cbz"
            match = re.search(r'Ch\.([\d.]+)', cbz_file.stem)
            if match:
                try:
                    existing_chapters.add(float(match.group(1)))
                except ValueError:
                    pass

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
            chapter_num = extract_chapter_number(chapter_url)

            # Skip if chapter already exists
            if chapter_num in existing_chapters:
                continue

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

            total_chapters += 1

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
                img_url = quote(img_url, safe=':/')
                # Get extension
                parsed = urlparse(img_url)
                path = parsed.path
                ext = path.split('.')[-1].lower()
                if ext not in ['jpg', 'jpeg', 'png', 'webp']:
                    ext = 'jpg'  # default
                filename = chapter_folder / f"{i:03d}"
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