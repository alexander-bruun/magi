#!/usr/bin/env python3
import json
"""
Tritinia scraper for MAGI.

Downloads manga/manhwa/manhua from tritinia.org.
"""

# Standard library imports
import os
import re
import shutil
import sys
from pathlib import Path
from urllib.parse import quote

# Third-party imports
import requests

# Local imports
from scraper_utils import (
    calculate_padding_width,
    convert_to_webp,
    create_cbz,
    check_duplicate_series,
    get_priority_config,
    error,
    format_chapter_name,
    get_existing_chapters,
    get_image_extension,
    get_session,
    log,
    log_existing_chapters,
    sanitize_title,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
DRY_RUN = os.getenv('dry_run', 'false').lower() == 'true'
CONVERT_TO_WEBP = os.getenv('convert_to_webp', 'true').lower() == 'true'
FOLDER = os.getenv('folder', os.path.join(os.path.dirname(__file__), 'Tritinia'))
DEFAULT_SUFFIX = os.getenv('default_suffix', '[Tritinia]')
ALLOWED_DOMAINS = ['tritinia.org']
BASE_URL = 'https://tritinia.org'
PRIORITY, HIGHER_PRIORITY_FOLDERS = get_priority_config('tritinia')


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from the listing page with load more.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series URLs, bool is_last_page)
    """
    if page_num == 1:
        # First page: direct fetch
        url = "https://tritinia.org/manga/"
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text
    else:
        # Subsequent pages: load more via AJAX
        ajax_url = "https://tritinia.org/wp-admin/admin-ajax.php"
        data = {
            'action': 'madara_load_more',
            'page': page_num,
            'template': 'madara-core/content/content-archive',
            'vars[paged]': page_num,
            'vars[orderby]': 'meta_value_num',
            'vars[template]': 'archive',
            'vars[sidebar]': 'right',
            'vars[post_type]': 'wp-manga',
            'vars[post_status]': 'publish',
            'vars[meta_key]': '_latest_update',
            'vars[order]': 'desc',
            'vars[meta_query][relation]': 'AND',
            'vars[manga_archives_item_layout]': 'default'
        }
        response = session.post(ajax_url, data=data, timeout=30)
        response.raise_for_status()
        html = response.text
    
    # Extract series URLs
    series_urls = re.findall(r'href="https://tritinia\.org/manga/[^"]*/"', html)
    # Remove href=" and " and filter out chapter and feed URLs
    series_urls = [url.replace('href="', '').rstrip('"') for url in series_urls 
                   if '/chapter-' not in url and '/ch-' not in url and '/feed/' not in url]
    
    is_last_page = len(series_urls) == 0
    return sorted(set(series_urls)), is_last_page

def extract_series_title(session, series_url):
    """
    Extract series title from the series page.

    Args:
        session: requests.Session object
        series_url: URL of the series page

    Returns:
        str: Series title, or None if not found
    """
    from scraper_utils import MAX_RETRIES, RETRY_DELAY
    import time
    
    for i in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text
            
            title_match = re.search(r'<title>([^<]+)', html)
            if title_match:
                title = title_match.group(1).replace(' &#8211; Tritinia Scans', '').strip()
                if title:
                    return title
        except Exception as e:
            if i < MAX_RETRIES:
                warn(f"Failed to extract title (attempt {i}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}")
                time.sleep(RETRY_DELAY)
    
    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    ajax_url = f"{series_url}ajax/chapters/"
    headers = {
        'accept': '*/*',
        'accept-language': 'en-GB,en-US;q=0.9,en;q=0.8',
        'x-requested-with': 'XMLHttpRequest',
        'origin': 'https://tritinia.org',
        'referer': series_url,
        'sec-fetch-dest': 'empty',
        'sec-fetch-mode': 'cors',
        'sec-fetch-site': 'same-origin',
    }
    response = session.post(ajax_url, headers=headers, timeout=30)
    response.raise_for_status()
    html = response.text
    
    # Extract chapter URLs
    chapter_urls = re.findall(r'href="https://tritinia\.org/manga/[^"]*ch-[^"]*/"', html)
    chapter_urls = [url.replace('href="', '').rstrip('"') for url in chapter_urls]
    
    # Sort numerically by chapter number
    unique_urls = list(set(chapter_urls))
    unique_urls.sort(key=lambda x: int(re.search(r'ch-(\d+)', x).group(1)) if re.search(r'ch-(\d+)', x) else 0)
    return unique_urls

def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: URL of the chapter page

    Returns:
        list: List of image URLs
    """
    import json
    
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace('\n', ' ')
    
    # Extract image URLs from chapter_preloaded_images script
    script_match = re.search(r'chapter_preloaded_images = (\[[\s\S]*?\])', html)
    if script_match:
        try:
            image_urls = json.loads(script_match.group(1))
            # Remove duplicates while preserving order
            return list(dict.fromkeys(image_urls))
        except json.JSONDecodeError:
            pass
    
    # Fallback: Extract image URLs from data-src attributes
    image_urls = re.findall(r'data-src="\s*(https://tritinia\.org/wp-content/uploads/WP-manga/data/[^"]*\.(?:jpg|jpeg|png|webp))', html)
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))
    
    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(r'src="\s*(https://tritinia\.org/wp-content/uploads/WP-manga/data/[^"]*\.(?:jpg|jpeg|png|webp))', html)
        image_urls = list(dict.fromkeys(image_urls))
    
    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Tritinia scraper")
    log("Mode: Full Downloader")

    # Create session and get cookies by visiting the site
    session = get_session()
    log(f"Performing health check on {BASE_URL}...")
    try:
        # Visit main site
        response = session.get("https://tritinia.org", timeout=30)
        if response.status_code != 200:
            error(f"Failed to get initial cookies. Returned {response.status_code}")
            return
        # Visit a sample series page to get manga-specific cookies
        response = session.get("https://tritinia.org/manga/blue-period/", timeout=30)
        if response.status_code != 200:
            error(f"Failed to get manga cookies. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Failed to initialize session: {e}")
        return

    success("Session initialized")

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
        # Check for duplicate in higher priority providers
        if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
            continue

        # Extract chapter URLs
        try:
            chapter_urls = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        # In dry run mode, only check first few chapters to see if series has content
        original_count = len(chapter_urls)
        if DRY_RUN and original_count > 5:
            chapter_urls = chapter_urls[:5]  # Only check first 5 chapters in dry run
            log(f"Dry run: checking only first 5 of {original_count} chapters")

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

        # Create series directory (only after confirming chapters with images exist)
        series_directory = Path(FOLDER) / clean_title
        series_directory.mkdir(parents=True, exist_ok=True)

        # Extract chapter numbers for padding and skipping logic
        chapter_nums = []
        for url in chapter_urls:
            match = re.search(r'ch-([^/]+)', url)
            if match:
                try:
                    chapter_nums.append(int(match.group(1)))
                except ValueError:
                    continue
        
        if not chapter_nums:
            warn(f"No valid chapter numbers found for {title}, skipping...")
            continue

        max_chapter = max(chapter_nums)
        padding_width = calculate_padding_width(max_chapter)
        log(f"Found {len(chapter_urls)} chapters (max: {max_chapter}, padding: {padding_width})")

        # Check for existing chapters
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

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
            chapter_num_match = re.search(r'ch-([^/]+)', chapter_url)
            if not chapter_num_match:
                continue
            try:
                chapter_num = int(chapter_num_match.group(1))
            except ValueError:
                continue

            # Skip if chapter already exists
            if chapter_num in existing_chapters:
                continue

            chapter_name = format_chapter_name(clean_title, chapter_num, padding_width, DEFAULT_SUFFIX)

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

            log(f"Downloading: Chapter {chapter_num} [{len(image_urls)} images]")

            # Create chapter directory
            chapter_folder = series_directory / chapter_name
            chapter_folder.mkdir(parents=True, exist_ok=True)

            # Download images
            downloaded_count = 0
            for i, img_url in enumerate(image_urls, 1):
                if not img_url:
                    continue
                ext = get_image_extension(img_url, 'jpg')
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

            # Only create CBZ if more than 1 image was downloaded
            if downloaded_count > 1:
                if create_cbz(chapter_folder, chapter_name):
                    # Remove temp folder
                    shutil.rmtree(chapter_folder)
                else:
                    warn(f"CBZ creation failed for {chapter_title}, keeping folder")
            else:
                log(f"Skipping CBZ creation for {chapter_title} - only {downloaded_count} image(s) downloaded")
                # Remove temp folder
                shutil.rmtree(chapter_folder)

    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {FOLDER}")


if __name__ == '__main__':
    main()