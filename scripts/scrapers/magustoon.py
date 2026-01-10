#!/usr/bin/env python3
"""
MagusToon scraper for MAGI.

Downloads manga/manhwa/manhua from magustoon.org.
Uses API for series listing and chapter metadata.
"""

# Standard library imports
import asyncio
import re
import shutil
import time
from pathlib import Path

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    calculate_padding_width,
    check_duplicate_series,
    create_cbz,
    download_images_to_folder,
    error,
    format_chapter_name,
    get_existing_chapters,
    get_scraper_config,
    get_session,
    log_existing_chapters,
    log_scraper_summary,
    log,
    MAX_RETRIES,
    RETRY_DELAY,
    run_scraper,
    sanitize_title,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("magustoon", "MagusToon", "[MagusToon]")
ALLOWED_DOMAINS = ["storage.magustoon.org"]
BASE_URL = "https://magustoon.org"
API_BASE_URL = "https://api.magustoon.org"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series URLs from API listing.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    all_series = []
    current_page = 1

    while True:
        api_url = (
            f"{API_BASE_URL}/api/query?page={current_page}&perPage=24&seriesType=&seriesStatus="
        )
        log(f"Fetching series list from page {current_page}...")

        try:
            response = session.get(api_url, timeout=30)
            response.raise_for_status()
            data = response.json()

            # Check if we have data
            posts_list = data.get("posts", [])
            if not posts_list:
                break

            # Extract series slugs
            for post in posts_list:
                if post.get("slug"):
                    all_series.append({'series_url': f"/series/{post.get('slug')}"})

            # Check if this is the last page
            total = data.get("total", 0)
            per_page = 24
            if (current_page * per_page) >= total:
                break

            current_page += 1

        except Exception as e:
            error(f"Error fetching page {current_page}: {e}")
            break

    return all_series


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract title from <title> tag
            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = title_match.group(1).strip()
                # Remove common suffixes
                title = re.sub(
                    r"\s*(?:Manga\s*)?-\s*Magus\s+Manga.*$", "", title
                ).strip()
                title = re.sub(r"\s*[-|]\s*MagusToon.*$", "", title).strip()
                if title:
                    return title

            # Try to extract from Next.js script data (postTitle)
            # Look for "postTitle":"Series Name" in the script tag
            title_match = re.search(r'"postTitle":"([^"]+)"', html)
            if title_match:
                return title_match.group(1).strip()

        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract title (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract title after {MAX_RETRIES} attempts: {e}")
                return None

    return None


def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    series_slug = series_url.split("/")[-1]
    full_url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract self.__next_f.push data blocks
            next_f_matches = re.findall(
                r"self\.__next_f\.push\(\[(.*?)\]\)", html, re.DOTALL
            )

            chapters = []
            seen_numbers = set()

            # Search through all __next_f data blocks for chapter information
            for match in next_f_matches:
                # Try Pattern 1: Double-escaped with featured image
                chapter_objects = re.findall(
                    r"\\\\\"slug\\\\\":\\\\\"(chapter-[^\\\\\"]+)\\\\\"[^}]*?\\\\\"number\\\\\":(\d+)[^}]*?\\\\\"featuredImage\\\\\":\\\\\"([^\\\\\"]+)\\\\\"",
                    match,
                )

                # Try Pattern 2: Single-escaped with featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r"\\\"slug\\\":\\\"(chapter-[^\\\"]+)\\\"[^}]*?\\\"number\\\":(\d+)[^}]*?\\\"featuredImage\\\":\\\"([^\\\"]+)\\\"",
                        match,
                    )

                # Try Pattern 3: Backslash-quote format with featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r'\\"slug\\":\\"(chapter-\d+)\\"[^}]*?\\"number\\":(\d+)[^}]*?\\"featuredImage\\":\\"([^\\"]+)\\"',
                        match,
                    )

                # Try Pattern 4: Any format without requiring featured image - just slug and number
                if not chapter_objects:
                    # Double-escaped
                    chapter_objects = re.findall(
                        r"\\\\\"slug\\\\\":\\\\\"(chapter-\d+)\\\\\"[^}]*?\\\\\"number\\\\\":(\d+)",
                        match,
                    )
                    # Add empty featured_image for consistency
                    chapter_objects = [(slug, num, "") for slug, num in chapter_objects]

                # Try Pattern 5: Single-escaped without featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r"\\\"slug\\\":\\\"(chapter-\d+)\\\"[^}]*?\\\"number\\\":(\d+)",
                        match,
                    )
                    chapter_objects = [(slug, num, "") for slug, num in chapter_objects]

                # Try Pattern 6: Backslash-quote format without featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r'\\"slug\\":\\"(chapter-\d+)\\"[^}]*?\\"number\\":(\d+)', match
                    )
                    chapter_objects = [(slug, num, "") for slug, num in chapter_objects]

                for slug, number, featured_image in chapter_objects:
                    try:
                        num = int(number)
                        if num in seen_numbers:
                            continue
                        seen_numbers.add(num)

                        chapter_url = f"{BASE_URL}/series/{series_slug}/{slug}"
                        chapters.append({'url': chapter_url, 'num': num})
                    except ValueError:
                        continue

            if chapters:
                chapters.sort(key=lambda x: x["num"])
                return chapters
            else:
                return []

        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract chapters (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract chapters after {MAX_RETRIES} attempts: {e}")
                return []

    return []


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Poster URL, or None if not found
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image with object-cover class
    poster_match = re.search(r'<img[^>]*class="[^"]*object-cover[^"]*"[^>]*src="([^"]+)"', html, re.IGNORECASE)
    if poster_match:
        return poster_match.group(1)

    # Fallback: look for any image with storage.magustoon.org domain
    img_match = re.search(r'<img[^>]*src="(https://storage\.magustoon\.org/[^"]*\.(?:jpg|png|webp))"', html, re.IGNORECASE)
    if img_match:
        return img_match.group(1)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_data(session, series_url):
    """
    Extract ALL chapter data from series page using requests.
    Parses the self.__next_f.push data to find embedded chapter list.
    Handles escaped quotes in the JSON data.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series (e.g., '/series/series-slug')

    Returns:
        list: Chapter data dictionaries with slug, number, title, featured_image
    """
    series_slug = series_url.split("/")[-1]
    full_url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract self.__next_f.push data blocks
            next_f_matches = re.findall(
                r"self\.__next_f\.push\(\[(.*?)\]\)", html, re.DOTALL
            )

            chapters = []
            seen_numbers = set()

            # Search through all __next_f data blocks for chapter information
            for match in next_f_matches:
                # Try Pattern 1: Double-escaped with featured image
                chapter_objects = re.findall(
                    r"\\\\\"slug\\\\\":\\\\\"(chapter-[^\\\\\"]+)\\\\\"[^}]*?\\\\\"number\\\\\":(\d+)[^}]*?\\\\\"featuredImage\\\\\":\\\\\"([^\\\\\"]+)\\\\\"",
                    match,
                )

                # Try Pattern 2: Single-escaped with featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r"\\\"slug\\\":\\\"(chapter-[^\\\"]+)\\\"[^}]*?\\\"number\\\":(\d+)[^}]*?\\\"featuredImage\\\":\\\"([^\\\"]+)\\\"",
                        match,
                    )

                # Try Pattern 3: Backslash-quote format with featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r'\\"slug\\":\\"(chapter-\d+)\\"[^}]*?\\"number\\":(\d+)[^}]*?\\"featuredImage\\":\\"([^\\"]+)\\"',
                        match,
                    )

                # Try Pattern 4: Any format without requiring featured image - just slug and number
                if not chapter_objects:
                    # Double-escaped
                    chapter_objects = re.findall(
                        r"\\\\\"slug\\\\\":\\\\\"(chapter-\d+)\\\\\"[^}]*?\\\\\"number\\\\\":(\d+)",
                        match,
                    )
                    # Add empty featured_image for consistency
                    chapter_objects = [(slug, num, "") for slug, num in chapter_objects]

                # Try Pattern 5: Single-escaped without featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r"\\\"slug\\\":\\\"(chapter-\d+)\\\"[^}]*?\\\"number\\\":(\d+)",
                        match,
                    )
                    chapter_objects = [(slug, num, "") for slug, num in chapter_objects]

                # Try Pattern 6: Backslash-quote format without featured image
                if not chapter_objects:
                    chapter_objects = re.findall(
                        r'\\"slug\\":\\"(chapter-\d+)\\"[^}]*?\\"number\\":(\d+)', match
                    )
                    chapter_objects = [(slug, num, "") for slug, num in chapter_objects]

                for slug, number, featured_image in chapter_objects:
                    try:
                        num = int(number)
                        if num in seen_numbers:
                            continue
                        seen_numbers.add(num)

                        chapters.append(
                            {
                                "slug": slug,
                                "number": num,
                                "title": "",
                                "featured_image": featured_image,
                            }
                        )
                    except ValueError:
                        continue

            if chapters:
                chapters.sort(key=lambda x: x["number"], reverse=True)
                return chapters
            else:
                return []

        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract chapters (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract chapters after {MAX_RETRIES} attempts: {e}")
                return []

    return []


def extract_image_urls(session, series_slug, chapter_slug, featured_image_url):
    """
    Extract image URLs for a chapter by fetching the chapter page.

    Args:
        session: requests.Session object
        series_slug: Series slug
        chapter_slug: Chapter slug
        featured_image_url: The featured image URL for the chapter (unused)

    Returns:
        list: Image URLs in reading order, or empty list if premium/inaccessible
    """
    # Build chapter URL
    chapter_url = f"{BASE_URL}/series/{series_slug}/{chapter_slug}"

    try:
        response = session.get(chapter_url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if chapter is premium
        if "This premium chapter is waiting to be unlocked" in html:
            log(f"    Chapter {chapter_slug} is premium, skipping")
            return []

        # Extract image URLs from the page
        # Look for chapter page images: /upload/series/{series}/{hash}/{page}.webp
        image_urls = re.findall(
            r'https://storage\.magustoon\.org/+upload/series/[^"\'<>]+\.webp', html
        )

        # Remove duplicates while preserving order
        seen = set()
        unique_urls = []
        for url in image_urls:
            if url not in seen:
                seen.add(url)
                unique_urls.append(url)

        return unique_urls

    except Exception as e:
        warn(f"Failed to extract images for {chapter_slug}: {e}")
        return []


# =============================================================================
# Download Functions
# =============================================================================
def download_chapter(session, series_info, chapter_info, chapter_dir):
    """
    Download all images for a chapter.

    Args:
        session: requests.Session object
        series_info: dict with series information
        chapter_info: dict with chapter information
        chapter_dir: Path to save chapter images

    Returns:
        bool: True if successful, False otherwise
    """
    series_slug = series_info["slug"]
    chapter_slug = chapter_info["slug"]
    featured_image = chapter_info.get("featured_image", "")
    chapter_num = chapter_info["number"]

    image_urls = extract_image_urls(session, series_slug, chapter_slug, featured_image)

    if not image_urls:
        log(f"Skipping: Chapter {chapter_num} (no images)")
        return False

    log(f"Downloading: {chapter_name} [{len(image_urls)} images]")

    # Download images using shared function
    result = download_images_to_folder(
        image_urls,
        chapter_dir,
        session,
        options={
            "convert_webp": CONVERT_TO_WEBP,
            "concurrent": False,
            "timeout": 30,
        },
    )

    return result["downloaded"] > 0


# =============================================================================
# Main Entry Point
# =============================================================================
async def setup_session_with_bypass():
    """Setup session with Cloudflare bypass if available."""
    try:
        cookies, headers = await bypass_cloudflare(BASE_URL)
        if cookies and "cf_clearance" in cookies:
            session = get_session(cookies, headers)
            success("Cloudflare bypass successful")
            return session
    except Exception as e:
        warn(f"Cloudflare bypass failed: {e}")

    return get_session()


def run_sync(session):
    """Synchronous requests-based scraper."""
    log("Starting MagusToon scraper")
    log("Mode: Full Downloader")

    output_dir = Path(CONFIG["folder"])
    output_dir.mkdir(exist_ok=True)
    processed_series = 0
    total_chapters = 0
    page_num = 1

    while True:
        try:
            series_urls, is_last_page = extract_series_urls(session, page_num)
            if not series_urls:
                if is_last_page:
                    break
                page_num += 1
                continue

            log(f"Found {len(series_urls)} series on page {page_num}")
            result = scrape_series(session, series_urls, output_dir)
            processed_series += result["series_count"]
            total_chapters += result["chapter_count"]
            page_num += 1

            if is_last_page:
                break

        except Exception as e:
            error(f"Error on page {page_num}: {e}")
            page_num += 1
            if page_num > 100:
                break

    log_scraper_summary(processed_series, total_chapters, CONFIG)


def scrape_series(session, series_urls, output_dir):
    """Scrape series using requests-based extraction only."""
    processed = 0
    total_chapters = 0

    for series_url in series_urls:
        series_slug = series_url.split("/")[-1]
        log(f"Processing: {series_url}")

        title = extract_series_title(session, series_url)
        if not title:
            continue

        title = sanitize_title(title)
        log(f"Title: {title}")

        # Check for duplicate in higher priority providers
        if check_duplicate_series(title, CONFIG["higher_priority_folders"]):
            continue

        series_info = {
            "slug": series_slug,
            "title": title,
            "url": f"{BASE_URL}{series_url}",
        }

        # Extract chapters using requests
        chapters = extract_chapter_data(session, series_url)

        if not chapters:
            warn(f"No chapters found for {title}, skipping...")
            continue

        # Create series directory
        series_output_dir = output_dir / f"{title} {CONFIG['default_suffix']}"
        series_output_dir.mkdir(parents=True, exist_ok=True)

        # Get chapter info for logging
        max_chapter = max(ch["number"] for ch in chapters)
        padding_width = calculate_padding_width(max_chapter)
        log(
            f"Found {len(chapters)} chapters (max: {max_chapter}, padding: {padding_width})"
        )

        # Check for existing chapters
        existing_chapters = get_existing_chapters(series_output_dir)
        log_existing_chapters(existing_chapters)

        for chapter_info in chapters:
            chapter_num = chapter_info["number"]
            padding_width = calculate_padding_width(max_chapter)
            chapter_name = format_chapter_name(
                title, chapter_num, padding_width, CONFIG["default_suffix"]
            )
            chapter_dir = series_output_dir / chapter_name

            if chapter_dir.exists() and list(chapter_dir.glob("*")):
                continue

            chapter_dir.mkdir(parents=True, exist_ok=True)

            if CONFIG["dry_run"]:
                image_urls = extract_image_urls(
                    get_session(),
                    series_slug,
                    chapter_info["slug"],
                    chapter_info.get("featured_image", ""),
                )
                log(
                    f"Chapter {chapter_num} [{len(image_urls)} images]"
                    if image_urls
                    else f"Chapter {chapter_num} [0 images - would skip]"
                )
                continue

            if download_chapter(session, series_info, chapter_info, chapter_dir):
                try:
                    create_cbz(chapter_dir, series_output_dir / f"{chapter_name}.cbz")
                    shutil.rmtree(chapter_dir)
                    total_chapters += 1
                except Exception as e:
                    error(f"Failed to create CBZ: {e}")
            else:
                try:
                    shutil.rmtree(chapter_dir)
                except Exception:
                    pass

        processed += 1

    return {"series_count": processed, "chapter_count": total_chapters}


def main():
    """Main entry point for the scraper."""
    run_scraper(
        extract_series_urls,
        extract_chapter_urls,
        extract_image_urls,
        extract_series_title,
        extract_poster_url,
        CONFIG,
        ALLOWED_DOMAINS,
        cloudflare_bypass=True,
    )


if __name__ == "__main__":
    main()
