#!/usr/bin/env python3
"""
Flame Comics scraper for MAGI.

Downloads manga/manhwa/manhua from flamecomics.xyz.
"""

# Standard library imports
import re
import time
import requests
import json
import urllib.parse

# Local imports
from scraper_utils import (
    error,
    get_default_headers,
    get_scraper_config,
    log,
    MAX_RETRIES,
    RETRY_DELAY,
    run_scraper,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("flamecomics", "FlameComics", "[FlameComics]")
ALLOWED_DOMAINS = ["cdn.flamecomics.xyz"]
BASE_URL = "https://flamecomics.xyz"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series data from browse page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, total_pages)
    """
    # Flame Comics doesn't have pagination, just one browse page
    if page_num > 1:
        return [], 1

    url = f"{BASE_URL}/browse"
    log("Fetching series from browse page...")

    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract series URLs (numeric IDs)
    series_urls = re.findall(r'href="/series/(\d+)"', html)
    full_urls = [f"{BASE_URL}/series/{sid}" for sid in series_urls]
    
    # Remove duplicates
    full_urls = sorted(set(full_urls))
    log(f"Found {len(full_urls)} total series")

    # Convert to dicts with series_url
    series_data = [{'series_url': url} for url in full_urls]
    return series_data, 1


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text

            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = title_match.group(1).replace(" - Flame Comics", "").strip()
                if title:
                    return title
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract title (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)

    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Poster URL or None
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Look for poster image with specific class
        poster_match = re.search(r'<img[^>]*class="SeriesPage_cover__cEjW-"[^>]*src="([^"]+)"', html)
        if poster_match:
            src = poster_match.group(1)
            # Parse the Next.js image URL to extract the actual image URL
            parsed = urllib.parse.urlparse(src)
            query = urllib.parse.parse_qs(parsed.query)
            if 'url' in query:
                poster_url = query['url'][0]
                return poster_url

        return None

    except Exception as e:
        error(f"Error extracting poster from {series_url}: {e}")
        return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract __NEXT_DATA__ JSON
    json_match = re.search(
        r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL
    )
    if not json_match:
        error(f"Could not find __NEXT_DATA__ in {series_url}")
        return []

    try:
        data = json.loads(json_match.group(1))
        chapters = data.get("props", {}).get("pageProps", {}).get("chapters", [])

        # Sort by chapter number
        chapters.sort(key=lambda x: float(x.get("chapter", 0)))

        chapter_info = []
        seen_nums = set()
        for chapter in chapters:
            series_id = chapter.get("series_id")
            token = chapter.get("token")
            chapter_num = float(chapter.get("chapter", 0))
            if series_id and token and chapter_num not in seen_nums:
                url = f"{BASE_URL}/series/{series_id}/{token}"
                chapter_info.append({'url': url, 'num': chapter_num})
                seen_nums.add(chapter_num)

        return chapter_info
    except (json.JSONDecodeError, KeyError) as e:
        error(f"Error parsing JSON from {series_url}: {e}")
        return []


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs in reading order, empty list if unavailable
    """
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(chapter_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract __NEXT_DATA__ JSON
            json_match = re.search(
                r'<script id="__NEXT_DATA__"[^>]*>(.*?)</script>', html, re.DOTALL
            )
            if not json_match:
                if attempt < MAX_RETRIES:
                    time.sleep(RETRY_DELAY)
                continue

            data = json.loads(json_match.group(1))
            chapter_data = data.get("props", {}).get("pageProps", {}).get("chapter", {})
            images = chapter_data.get("images", {})
            series_id = chapter_data.get("series_id")
            token = chapter_data.get("token")

            if not series_id or not token:
                if attempt < MAX_RETRIES:
                    time.sleep(RETRY_DELAY)
                continue

            urls = []
            for key, img_data in images.items():
                name = img_data.get("name", "")
                if "commission" not in name:
                    url = f"https://cdn.flamecomics.xyz/uploads/images/series/{series_id}/{token}/{name}"
                    urls.append(url)

            if urls:
                return urls
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract images (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)

    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    try:
        log("Starting Flame Comics scraper")
        log("Mode: Full Downloader")

        # Health check
        log(f"Performing health check on {BASE_URL}...")
        try:
            session = requests.Session()
            session.headers.update(get_default_headers())

            response = session.get(BASE_URL, timeout=30)
            if response.status_code != 200:
                error(f"Health check failed. Returned {response.status_code}")
                return
        except Exception as e:
            error(f"Health check failed: {e}")
            return

        success("Health check passed")

        # Run the scraper
        run_scraper(
            session=session,
            config=CONFIG,
            extract_series_func=extract_series_urls,
            extract_series_title_func=extract_series_title,
            extract_chapter_urls_func=extract_chapter_urls,
            extract_image_urls_func=extract_image_urls,
            extract_poster_func=extract_poster_url,
            allowed_domains=ALLOWED_DOMAINS,
            base_url=BASE_URL,
            series_url_builder=lambda data: data['series_url']
        )
    except Exception as e:
        error(f"Unexpected error in main(): {e}")
        import traceback

        traceback.print_exc()


if __name__ == "__main__":
    main()
