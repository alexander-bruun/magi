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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'MangaYY'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[MangaYY]')
ALLOWED_DOMAINS = ['mangayy.org', 'yy.mangayy.org']
BASE_URL = 'https://mangayy.org'

# Extract series URLs from the manga listing page
def extract_series_urls(session, page_num):
    url = f"https://mangayy.org/page/{page_num}/?m_orderby=new-manga"
    log(f"Fetching series list from page {page_num}...")
    
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Check if this is the last page by looking for "next" link
    is_last_page = 'next page-numbers' not in html and 'Next' not in html
    
    # Extract series URLs - look for manga entry links
    series_urls = re.findall(r'href="https://mangayy\.org(/manga/[^/]+/)"', html)
    # Filter out chapter URLs and other non-series URLs
    series_urls = [url for url in series_urls if 'chapter' not in url and 'feed' not in url and 'genre' not in url]
    return sorted(set(series_urls)), is_last_page

# Extract series title from series page
def extract_series_title(session, series_url):
    url = f"https://mangayy.org{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        import html
        title = html.unescape(title_match.group(1))
        title = title.replace(' – MangaYY', '').strip()
        return title
    return None

# Extract chapter URLs for a given manga
def extract_chapter_urls(session, manga_url):
    full_url = f"https://mangayy.org{manga_url}"
    
    # First get the manga page to extract the post ID or other needed data
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract manga slug from URL
    manga_slug = manga_url.strip('/').split('/')[-1]
    
    # Try to get chapters via AJAX
    ajax_url = f"https://mangayy.org{manga_url}ajax/chapters/?t=1"
    try:
        ajax_response = session.post(ajax_url, timeout=30)
        ajax_response.raise_for_status()
        ajax_html = ajax_response.text
        
        # Extract chapter URLs from AJAX response
        chapter_urls = re.findall(r'href="https://mangayy\.org(/manga/' + re.escape(manga_slug) + r'/chapter-[^/]+/)"', ajax_html)
        
        if chapter_urls:
            # Remove duplicates and sort by chapter number
            unique_urls = sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
            return unique_urls
    except Exception as e:
        warn(f"AJAX chapter loading failed: {e}, falling back to HTML parsing")
    
    # Fallback: extract from the original HTML (for sites that don't use AJAX)
    chapter_urls = re.findall(r'href="https://mangayy\.org(' + re.escape(manga_url.rstrip('/')) + r'/chapter-[^/]+/)"', html)
    # Remove duplicates and sort by chapter number
    unique_urls = sorted(set(chapter_urls), key=lambda x: int(re.search(r'chapter-(\d+)', x).group(1)) if re.search(r'chapter-(\d+)', x) else 0)
    return unique_urls

# Extract image URLs for a given chapter
def extract_image_urls(session, chapter_url):
    full_url = f"https://mangayy.org{chapter_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\0', '')
    
    # Look for img src attributes that contain manga images
    image_urls = re.findall(r'<img[^>]*src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
    # Clean URLs and filter for manga images from yy.mangayy.org
    cleaned_urls = []
    for url in image_urls:
        url = url.strip()
        if ('yy.mangayy.org' in url or 'WP-manga' in url) and 'thumbnails' not in url:
            # Remove any leading/trailing whitespace or encoded characters
            url = re.sub(r'^[^h]*', '', url)  # Remove anything before 'http'
            if url.startswith('http'):
                cleaned_urls.append(url)
    return list(dict.fromkeys(cleaned_urls))  # unique

def main():
    log("Starting MangaYY scraper")
    log("Mode: Full Downloader")

    # Health check
    log("Performing health check on https://mangayy.org...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://mangayy.org", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Process series page by page to save progress incrementally
    total_series = 0
    total_chapters = 0
    page = 1
    while True:
        try:
            page_series, is_last_page = extract_series_urls(session, page)
            if not page_series:
                log(f"No series found on page {page}, stopping.")
                break
            
            log(f"Found {len(page_series)} series on page {page}")
            
            # Process each series on this page immediately
            for series_url in page_series:
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
                max_chapter = max(int(re.search(r'chapter-(\d+)', url).group(1)) for url in chapter_urls)
                padding_width = len(str(max_chapter))
                log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

                # Scan existing CBZ files to determine which chapters are already downloaded
                existing_chapters = set()
                for cbz_file in series_directory.glob("*.cbz"):
                    # Extract chapter number from filename like "Title Ch.001 [MangaYY].cbz"
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
                    chapter_num = int(re.search(r'chapter-(\d+)', chapter_url).group(1))

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
                    # Use a fresh session for image downloads (no Cloudflare bypass needed for image domain)
                    image_session = requests.Session()
                    image_session.headers.update({
                        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
                        'Referer': f"https://mangayy.org{chapter_url}"
                    })
                    for i, img_url in enumerate(image_urls, 1):
                        if not img_url or not any(domain in img_url for domain in ALLOWED_DOMAINS):
                            continue
                        # Don't re-encode URLs that are already encoded
                        # Get extension
                        parsed = urlparse(img_url)
                        path = parsed.path
                        ext = path.split('.')[-1].lower()
                        if ext not in ['jpg', 'jpeg', 'png', 'webp']:
                            ext = 'jpg'  # default
                        filename = chapter_folder / f"{i:03d}.{ext}"
                        try:
                            response = image_session.get(img_url, timeout=30)
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

                total_series += 1
                log(f"Completed series: {clean_title} ({len(chapter_urls)} chapters)")

            if is_last_page:
                log(f"Reached last page (page {page}).")
                break
            page += 1
            
        except Exception as e:
            error(f"Error processing page {page}: {e}")
            break

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")

if __name__ == "__main__":
    main()