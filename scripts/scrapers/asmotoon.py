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
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'AsmoToon'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[AsmoToon]')
ALLOWED_DOMAINS = ['cdn.meowing.org']
BASE_URL = 'https://asmotoon.com'

# Extract series URLs from listing page with pagination
def extract_series_urls(session, page_num):
    if page_num == 1:
        url = "https://asmotoon.com/series"
    else:
        url = f"https://asmotoon.com/series/page/{page_num}/"

    log(f"Fetching series list from page {page_num}...")
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Check if this is the last page
    is_last_page = 'next page-numbers' not in html and 'Next' not in html and '>' not in html

    # Extract series URLs
    series_urls = re.findall(r'href="(/series/[^/]+/)"', html)

    return sorted(set(series_urls)), is_last_page

# Extract series title from series page
def extract_series_title(session, series_url):
    url = f"https://asmotoon.com{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Check if this is a novel series (skip if so)
    novel_indicators = ['> novel <', 'novel series', 'text novel', 'light novel']
    if any(indicator in html for indicator in novel_indicators):
        return None

    title_match = re.search(r'<title>([^<]+)', html)
    if title_match:
        title = title_match.group(1).replace(' - Asmo Toon', '').strip()
        return title

    return None

# Extract chapter URLs from series page
def extract_chapter_urls(session, series_url):
    url = f"https://asmotoon.com{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Find all chapter URLs
    chapter_urls = re.findall(r'/chapter/[^\"]+', html)
    chapter_urls = list(set(chapter_urls))  # Remove duplicates
    
    chapters = []
    for chapter_url in chapter_urls:
        # Visit the chapter page to get the actual chapter number
        chapter_page_url = f"https://asmotoon.com{chapter_url}"
        try:
            chapter_response = session.get(chapter_page_url, timeout=30)
            chapter_response.raise_for_status()
            chapter_html = chapter_response.text
            
            # Look for chapter number in the page title or content
            title_match = re.search(r'<title>([^<]+)', chapter_html)
            if title_match:
                title = title_match.group(1)
                # Extract number from title like "Series Name - Chapter 7.1"
                number_match = re.search(r'Chapter\s+(\d+(?:\.\d+)?)', title, re.IGNORECASE)
                if number_match:
                    try:
                        chapter_num = float(number_match.group(1))
                        chapters.append((chapter_url, chapter_num))
                    except ValueError:
                        continue
        except Exception as e:
            # If we can't get the chapter page, skip this chapter
            continue
    
    # Sort by chapter number
    chapters.sort(key=lambda x: x[1])
    
    return chapters

# Extract image URLs from chapter page
def extract_image_urls(session, chapter_url):
    url = f"https://asmotoon.com{chapter_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Skip early access chapters
    if 'This is an early access chapter.' in html:
        return []

    # Look for the #pages div and extract img tags with uid attributes
    pages_div_match = re.search(r'<div[^>]*id="pages"[^>]*>(.*?)</div>', html, re.DOTALL | re.IGNORECASE)
    if pages_div_match:
        pages_html = pages_div_match.group(1)
        # Extract uid attributes from img tags
        uid_matches = re.findall(r'<img[^>]*uid="([^"]+)"[^>]*>', pages_html, re.IGNORECASE)

        image_urls = []
        for uid in uid_matches:
            if uid and len(uid.strip()) > 0:
                image_url = f"https://cdn.meowing.org/uploads/{uid}"
                image_urls.append(image_url)

        return image_urls

    # Fallback: look for image ID patterns in the entire HTML
    id_pattern = r'[A-Z][A-Za-z0-9]{9,11}'
    candidates = re.findall(id_pattern, html)

    # Filter candidates to ensure they have mixed case and numbers
    image_ids = []
    for candidate in candidates:
        has_upper = any(c.isupper() for c in candidate)
        has_lower = any(c.islower() for c in candidate)
        has_digit = any(c.isdigit() for c in candidate)
        if has_upper and has_lower and has_digit and len(candidate) >= 10:
            image_ids.append(candidate)

    # Remove duplicates while preserving order
    seen = set()
    unique_ids = []
    for img_id in image_ids:
        if img_id not in seen:
            seen.add(img_id)
            unique_ids.append(img_id)

    # Verify each ID forms a valid image URL
    valid_image_urls = []
    for img_id in unique_ids:
        test_url = f"https://cdn.meowing.org/uploads/{img_id}"
        try:
            img_resp = session.head(test_url, timeout=5)
            if img_resp.status_code == 200:
                valid_image_urls.append(test_url)
        except:
            pass

    return valid_image_urls

def main():
    log("Starting AsmoToon scraper")

    # Cloudflare bypass
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies or 'cf_clearance' not in cookies:
            warn("Cloudflare bypass failed, trying without bypass...")
            session = get_session()
        else:
            session = get_session(cookies, headers)
    except Exception as e:
        warn(f"Cloudflare bypass failed: {e}, trying without bypass...")
        session = get_session()

    # Health check
    try:
        response = session.get(BASE_URL, timeout=30)
        response.raise_for_status()
        success("Health check passed")
    except Exception as e:
        warn(f"Health check failed: {e}, continuing anyway...")

    # Create output directory
    output_dir = Path(FOLDER)
    output_dir.mkdir(exist_ok=True)

    page_num = 1
    processed_series = 0

    while True:
        try:
            series_urls, is_last_page = extract_series_urls(session, page_num)
            if not series_urls:
                if is_last_page:
                    log("No more series found, finishing...")
                    break
                else:
                    page_num += 1
                    continue

            log(f"Found {len(series_urls)} series on page {page_num}")

            for series_url in series_urls:
                try:
                    title = extract_series_title(session, series_url)
                    if not title:
                        warn(f"Could not extract title for {series_url}, skipping...")
                        continue

                    clean_title = sanitize_title(title)
                    log(f"Title: {title}")

                    series_directory = output_dir / f"{clean_title} {DEFAULT_SUFFIX}"
                    series_directory.mkdir(exist_ok=True)

                    chapters = extract_chapter_urls(session, series_url)
                    if not chapters:
                        warn(f"No chapters found for {title}, skipping...")
                        continue

                    # Extract chapter numbers for padding
                    chapter_nums = [chapter[1] for chapter in chapters]

                    if not chapter_nums:
                        # Fallback: use sequential numbering
                        chapter_nums = list(range(1, len(chapters) + 1))

                    max_chapter = max(chapter_nums)
                    # Calculate padding based on the integer part of the largest chapter number
                    padding_width = len(str(int(max_chapter)))
                    log(f"Found {len(chapters)} chapters (max: {max_chapter}, padding: {padding_width})")

                    # Scan existing CBZ files
                    existing_chapters = set()
                    for cbz_file in series_directory.glob("*.cbz"):
                        match = re.search(r'Chapter ([\d.]+)', cbz_file.stem)
                        if match:
                            existing_chapters.add(float(match.group(1)))

                    log(f"No existing chapters found, downloading all" if not existing_chapters else f"Found {len(existing_chapters)} existing chapters")

                    for chapter_url, chapter_num in chapters:
                        try:
                            if chapter_num in existing_chapters:
                                continue

                            # Format chapter number - handle decimals properly
                            if chapter_num == int(chapter_num):
                                formatted_chapter_number = f"{int(chapter_num):0{padding_width}d}"
                            else:
                                # For decimal numbers, only pad the integer part
                                integer_part = int(chapter_num)
                                decimal_part = chapter_num - integer_part
                                formatted_chapter_number = f"{integer_part:0{padding_width}d}.{int(decimal_part * 10)}"
                            
                            chapter_title = f"Chapter {formatted_chapter_number}"
                            chapter_name = f"{clean_title} {chapter_title} {DEFAULT_SUFFIX}"

                            log(f"Processing: {chapter_title}")

                            image_urls = extract_image_urls(session, chapter_url)
                            if not image_urls:
                                warn(f"No images found for {chapter_title}, skipping...")
                                continue

                            log(f"Found {len(image_urls)} images")

                            if DRY_RUN:
                                log(f"[DRY RUN] Would download {len(image_urls)} images for {chapter_title}")
                                continue

                            # Download images
                            chapter_folder = series_directory / f"{chapter_name}"
                            chapter_folder.mkdir(exist_ok=True)

                            downloaded_count = 0
                            for j, img_url in enumerate(image_urls):
                                try:
                                    img_response = session.get(img_url, timeout=30)
                                    img_response.raise_for_status()

                                    # Determine file extension
                                    content_type = img_response.headers.get('content-type', '')
                                    if 'webp' in content_type:
                                        ext = '.webp'
                                    elif 'png' in content_type:
                                        ext = '.png'
                                    else:
                                        ext = '.jpg'

                                    img_filename = f"{j+1:03d}{ext}"
                                    img_path = chapter_folder / img_filename

                                    with open(img_path, 'wb') as f:
                                        f.write(img_response.content)

                                    downloaded_count += 1
                                    print(f"  [{j+1:03d}] {img_url} Success")

                                    # Convert to WebP if needed
                                    if CONVERT_TO_WEBP and ext != '.webp':
                                        try:
                                            convert_to_webp(img_path)
                                        except Exception as e:
                                            # WebP conversion failed, keep original file
                                            pass

                                except Exception as e:
                                    error(f"Failed to download image {j+1}: {e}")

                            log(f"Downloaded: {chapter_title} [{downloaded_count}/{len(image_urls)} images]")

                            # Create CBZ
                            if create_cbz(chapter_folder, chapter_name):
                                success(f"Created {chapter_name}.cbz ({downloaded_count} files)")

                            # Clean up
                            import shutil
                            shutil.rmtree(chapter_folder)

                        except Exception as e:
                            error(f"Failed to process chapter {i+1}: {e}")

                    processed_series += 1

                except Exception as e:
                    error(f"Failed to process series {series_url}: {e}")

            if is_last_page:
                break
            page_num += 1

        except Exception as e:
            error(f"Failed to process page {page_num}: {e}")
            break

    log(f"Processed {processed_series} series")

if __name__ == "__main__":
    main()