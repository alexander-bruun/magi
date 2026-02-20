#!/usr/bin/env python3
"""
VortexScans scraper for MAGI.

Downloads manga/manhwa/manhua from vortexscans.org via their API.
"""

# Standard library imports
import asyncio
import os
import re

import json

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    error,
    get_scraper_config,
    get_session,
    log,
    run_scraper,
    success,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("vortexscans", "VortexScans", "[VortexScans]")
WEBP_QUALITY = int(os.getenv("webp_quality", "100"))
ALLOWED_DOMAINS = ["storage.vexmanga.com"]
API_CACHE_FILE = os.path.join(os.path.dirname(__file__), "vortexscans.json")
BASE_URL = "https://vortexscans.org"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract all series slugs from the API.

    Args:
        session: requests.Session object
        page_num: Page number (ignored, fetches all at once)

    Returns:
        tuple: (list of dicts with 'series_url' key, total_pages)
    """
    if page_num != 1:
        return [], 1
    
    if not os.path.exists(API_CACHE_FILE) or os.path.getsize(API_CACHE_FILE) == 0:
        log("Fetching all series data...")
        url = "https://api.vortexscans.org/api/query?page=1&perPage=99999"
        response = session.get(url, timeout=30)
        response.raise_for_status()
        data = response.json()
        with open(API_CACHE_FILE, "w") as f:
            json.dump(data, f)
    else:
        log("Loading series data from cache...")
        with open(API_CACHE_FILE, "r") as f:
            data = json.load(f)

    series_info = []
    for post in data.get("posts", []):
        slug = post.get("slug")
        if slug and not slug.startswith("chapter-"):
            series_info.append({'series_url': f"/series/{slug}"})

    return series_info, 1


def extract_series_title(session, series_url):
    """
    Extract series title from cached API data.

    Args:
        session: requests.Session object (not used)
        series_url: URL path of the series

    Returns:
        str: Series title, or None if not found or novel
    """
    # Extract slug from URL
    series_slug = series_url.split('/')[-1]
    
    with open(API_CACHE_FILE, "r") as f:
        data = json.load(f)

    for post in data.get("posts", []):
        if post.get("slug") == series_slug:
            title = post.get("postTitle", "")
            # Skip novels
            if "[Novel]" in title:
                return None
            return title

    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL path of the series

    Returns:
        str: Poster URL, or None if not found
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image with itemprop="image"
    poster_match = re.search(r'<img[^>]*itemprop="image"[^>]*src="([^"]+)"', html)
    if poster_match:
        return poster_match.group(1)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from the series page.

    Args:
        session: requests.Session object
        series_url: URL path of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", "")

    # Extract chapter slugs
    slugs = re.findall(r'\\"slug\\":\\"chapter-[^"]*\\"', html)
    chapter_info = []
    series_slug = series_url.split('/')[-1]
    for slug_match in slugs:
        slug = slug_match.replace('\\"slug\\":\\"', "").replace("\\", "").rstrip('"')
        chapter_url = f"/series/{series_slug}/{slug}"
        # Extract chapter number
        match = re.search(r"chapter-(\d+)", slug)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': chapter_url, 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: Chapter URL path

    Returns:
        list: List of image URLs
    """
    full_chapter_url = f"{BASE_URL}{chapter_url}"
    response = session.get(full_chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Check for premium - look for specific premium indicators
    if "This premium chapter is waiting to be unlocked" in html and (
        "purchase" in html or "coins" in html
    ):
        return []

    # Check for rate limiting
    if "Rate Limited" in html:
        return []

    # Extract image URLs
    img_urls = re.findall(
        r'https://storage\.vexmanga\.com/public/+upload/series/[^"]*\.(?:webp|jpg|jpeg|png)',
        html,
    )
    # Remove duplicates while preserving order
    img_urls = list(dict.fromkeys(img_urls))

    return img_urls


# ============================================================
# Main Entry Point
# ============================================================


def main():
    """Main entry point for the scraper."""
    log("Starting Vortex Scans scraper")

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
