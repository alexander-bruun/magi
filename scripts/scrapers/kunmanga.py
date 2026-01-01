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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'Kunmanga'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[Kunmanga]')
ALLOWED_DOMAINS = ['kunmanga.com', 'kunsv1.com', 'manimg24.com']
BASE_URL = 'https://kunmanga.com'

# Extract series URLs from listing page with pagination
def extract_series_urls(session, page_num):
    url = f"https://kunmanga.com/manga/page/{page_num}/?m_orderby=new-manga"
    response = session.get(url, timeout=30)
    
    # Check if page exists (404 means no more pages)
    if response.status_code == 404:
        return [], True  # is_last_page = True
    
    response.raise_for_status()
    html = response.text
    
    # Extract series URLs
    series_urls = re.findall(r'href="https://kunmanga\.com/manga/[^"]*/"', html)
    # Remove href=" and " and filter out chapter and feed URLs
    series_urls = [url.replace('href="', '').rstrip('"') for url in series_urls 
                   if '/chapter-' not in url and '/ch-' not in url and '/feed/' not in url and not url.endswith('/page/')]
    
    is_last_page = len(series_urls) == 0
    return sorted(set(series_urls)), is_last_page

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
                title = title_match.group(1).replace(' &#8211; Kunmanga', '').strip()
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
    ajax_url = f"{series_url}ajax/chapters/"
    response = session.post(ajax_url, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter URLs
    chapter_urls = re.findall(r'href="https://kunmanga\.com/manga/[^"]*chapter-[^"]*/"', html)
    chapter_urls = [url.replace('href="', '').rstrip('"') for url in chapter_urls]
    return sorted(set(chapter_urls))

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\n', ' ')
    
    # Extract image URLs from chapter_preloaded_images script
    script_match = re.search(r'chapter_preloaded_images = (\[[\s\S]*?\])', html)
    if script_match:
        import json
        try:
            image_urls = json.loads(script_match.group(1))
            # Filter to only include chapter images (kunsv1.com or manimg24.com)
            image_urls = [url for url in image_urls if 'kunsv1.com' in url or 'manimg24.com' in url]
            # Remove duplicates while preserving order
            return list(dict.fromkeys(image_urls))
        except json.JSONDecodeError:
            pass
    
    # Fallback: Extract image URLs from data-src attributes (multiple domains)
    image_urls = re.findall(r'data-src="\s*(https?://(?:kunmanga\.com|sv\d*\.kunsv1\.com|h\d*\.manimg24\.com)/[^"]*\.(?:jpg|jpeg|png|webp))', html)
    # Filter to only include chapter images (kunsv1.com or manimg24.com)
    image_urls = [url for url in image_urls if 'kunsv1.com' in url or 'manimg24.com' in url]
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))
    
    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(r'src="\s*(https?://(?:kunmanga\.com|sv\d*\.kunsv1\.com|h\d*\.manimg24\.com)/[^"]*\.(?:jpg|jpeg|png|webp))', html)
        # Filter to only include chapter images (kunsv1.com or manimg24.com)
        image_urls = [url for url in image_urls if 'kunsv1.com' in url or 'manimg24.com' in url]
        image_urls = list(dict.fromkeys(image_urls))
    
    return image_urls

def main():
    log("Starting Kunmanga scraper")
    log("Mode: Full Downloader")

    # Health check and bypass Cloudflare
    log("Performing health check on https://kunmanga.com...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://kunmanga.com", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

    success("Health check passed")

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Process series page by page, downloading as we go
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
                total_series += 1

                log(f"Title: {clean_title}")

                # Create series directory
                series_directory = Path(FOLDER) / clean_title
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

                # Check if this series has any chapters with images by testing the first few
                has_images = False
                test_chapters = chapter_urls[:min(5, len(chapter_urls))]  # Test first 5 chapters
                for test_url in test_chapters:
                    try:
                        if extract_image_urls(session, test_url):
                            has_images = True
                            break
                    except Exception:
                        continue
                
                if not has_images:
                    log(f"No chapters with images found for {title}, skipping...")
                    continue

                # Extract chapter numbers for padding and skipping logic
                chapter_nums = []
                for url in chapter_urls:
                    match = re.search(r'chapter-([^/]+)', url)
                    if match:
                        try:
                            chapter_nums.append(int(match.group(1)))
                        except ValueError:
                            continue
                
                if not chapter_nums:
                    warn(f"No valid chapter numbers found for {title}, skipping...")
                    continue

                max_chapter = max(chapter_nums)
                padding_width = len(str(max_chapter))
                log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

                # Scan existing CBZ files to determine which chapters are already downloaded
                existing_chapters = set()
                for cbz_file in series_directory.glob("*.cbz"):
                    # Extract chapter number from filename like "Title Chapter 001 [Kunmanga].cbz"
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
                    chapter_num_match = re.search(r'chapter-([^/]+)', chapter_url)
                    if not chapter_num_match:
                        continue
                    try:
                        chapter_num = int(chapter_num_match.group(1))
                    except ValueError:
                        continue

                    # Skip if chapter already exists
                    if chapter_num in existing_chapters:
                        continue

                    formatted_chapter_number = f"{chapter_num:0{padding_width}d}"
                    chapter_title = f"Chapter {formatted_chapter_number}"
                    chapter_name = f"{clean_title} {chapter_title} {DEFAULT_SUFFIX}"

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

                    log(f"Downloaded: Chapter {chapter_num} [{downloaded_count}/{len(image_urls)} images]")

                    # Only create CBZ if more than 1 image was downloaded
                    if downloaded_count > 1:
                        if create_cbz(chapter_folder, chapter_name):
                            # Remove temp folder
                            import shutil
                            shutil.rmtree(chapter_folder)
                        else:
                            warn(f"CBZ creation failed for {chapter_title}, keeping folder")
                    else:
                        log(f"Skipping CBZ creation for {chapter_title} - only {downloaded_count} image(s) downloaded")
                        # Remove temp folder
                        import shutil
                        shutil.rmtree(chapter_folder)

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