#!/usr/bin/env python3
"""
Thunder Scans scraper for MAGI.

Downloads manga/manhwa/manhua from en-thunderscans.com.
"""

# Standard library imports
import asyncio
import re

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    error,
    get_scraper_config,
    get_session,
    log,
    run_scraper,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("thunderscans", "ThunderScans", "[ThunderScans]")
ALLOWED_DOMAINS = ["en-thunderscans.com"]
BASE_URL = "https://en-thunderscans.com"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series URLs from all pages.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    all_series_urls = []
    page = 1
    while True:
        url = f"{BASE_URL}/comics/?page={page}"
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if this is the last page
        is_last_page = f'href="?page={page + 1}"' not in html or "No Post Found" in html

        # Extract series URLs
        series_urls = re.findall(r'href="https://en-thunderscans\.com/comics/[^"]*/"', html)
        series_urls = [url.replace('href="', "").rstrip('"') for url in series_urls]

        # Convert to dict format
        for series_url in series_urls:
            all_series_urls.append({'series_url': series_url})

        if is_last_page or not series_urls:
            break
        page += 1

    return sorted(set(all_series_urls), key=lambda x: x['series_url'])


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    title_match = re.search(r"<title>([^<]+)", html)
    if title_match:
        title = title_match.group(1).replace(" &#8211; Thunderscans EN", "").strip()
        return title

    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Poster URL, or None if not found
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image in the series page
    poster_match = re.search(r'<img[^>]*itemprop="image"[^>]*src="([^"]+)"', html, re.IGNORECASE)
    if poster_match:
        return poster_match.group(1)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter links from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter URLs
    chapter_urls = re.findall(
        r'href="https://en-thunderscans\.com/[^"]*chapter-[0-9]*/"', html
    )
    chapter_urls = [url.replace('href="', "").rstrip('"') for url in chapter_urls]

    # Convert to dict format with chapter numbers
    chapter_info = []
    for url in chapter_urls:
        match = re.search(r"chapter-(\d+)", url)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': url, 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs in reading order, empty list if locked/unavailable
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", "")

    # Check if chapter is locked
    if "This chapter is locked" in html or "lock-container" in html:
        warn("Chapter is locked, skipping...")
        return []

    # Extract images JSON
    images_match = re.search(r'"images":\[([^\]]*)\]', html)
    if not images_match:
        log("No images JSON found")
        return []

    images_json = images_match.group(1)
    # Extract URLs
    image_urls = re.findall(
        r'https://[^"]*\.(?:webp|jpg|png)', images_json.replace("\\", "")
    )

    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Thunder Scans scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
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
        series_url_builder=lambda data: data['series_url']  # data has 'series_url' key
    )


if __name__ == "__main__":
    main()
