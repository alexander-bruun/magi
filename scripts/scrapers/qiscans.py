#!/usr/bin/env python3
"""
QiScans scraper for MAGI.

Downloads manga/manhwa/manhua from qiscans.org via their API.
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
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("qiscans", "QiScans", "[QiScans]")
ALLOWED_DOMAINS = ["media.qiscans.org"]
API_CACHE_FILE = os.path.join(os.path.dirname(__file__), "qiscans.json")
BASE_URL = "https://qiscans.org"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series URLs from the API.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    if not os.path.exists(API_CACHE_FILE) or os.path.getsize(API_CACHE_FILE) == 0:
        log("Fetching all series data...")
        url = "https://api.qiscans.org/api/query?page=1&perPage=99999"
        response = session.get(url, timeout=30)
        response.raise_for_status()
        data = response.json()
        with open(API_CACHE_FILE, "w") as f:
            json.dump(data, f)
    else:
        log("Loading series data from cache...")
        with open(API_CACHE_FILE, "r") as f:
            data = json.load(f)

    series_urls = []
    for post in data.get("posts", []):
        slug = post.get("slug")
        if slug and not slug.startswith("chapter-"):
            series_url = f"/series/{slug}"
            series_urls.append({'series_url': series_url})

    return series_urls


def extract_series_title(session, series_url):
    """
    Extract series title from series URL.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Series title, or empty string if not found
    """
    # Extract series_slug from URL
    match = re.search(r'/series/([^/]+)', series_url)
    if not match:
        return ""
    
    series_slug = match.group(1)
    
    with open(API_CACHE_FILE, "r") as f:
        data = json.load(f)

    for post in data.get("posts", []):
        if post.get("slug") == series_slug:
            return post.get("postTitle", "")

    return ""


def get_series_id(series_slug):
    """
    Get series ID from cached API data.

    Args:
        series_slug: Slug of the series

    Returns:
        int: Series ID, or None if not found
    """
    with open(API_CACHE_FILE, "r") as f:
        data = json.load(f)

    for post in data.get("posts", []):
        if post.get("slug") == series_slug:
            return post.get("id")

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series URL.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    # Extract series_slug from URL
    match = re.search(r'/series/([^/]+)', series_url)
    if not match:
        return []
    
    series_slug = match.group(1)
    
    series_id = get_series_id(series_slug)
    if not series_id:
        warn(f"Could not find series ID for {series_slug}")
        return []

    # Use v2 API to get all chapters
    api_url = f"https://api.qiscans.org/api/v2/posts/{series_id}/chapters?page=1&perPage=9999&sortOrder=asc"
    response = session.get(api_url, timeout=30)
    response.raise_for_status()
    data = response.json()

    chapter_dicts = []
    for chapter in data.get("data", []):
        chapter_slug = chapter.get("slug")
        if chapter_slug:
            # Skip locked/inaccessible chapters
            if chapter.get("isLocked") or not chapter.get("isAccessible", True):
                continue
            chapter_url = f"{BASE_URL}/series/{series_slug}/{chapter_slug}"
            
            # Extract chapter number from slug
            num_match = re.search(r'chapter-(\d+)', chapter_slug)
            if num_match:
                chapter_num = int(num_match.group(1))
                chapter_dicts.append({'url': chapter_url, 'num': chapter_num})

    # Sort by chapter number
    chapter_dicts.sort(key=lambda x: x['num'])
    
    return chapter_dicts


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: URL of the chapter

    Returns:
        list: List of image URLs
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Check for premium
    if "This premium chapter is waiting to be unlocked" in html:
        return []

    # Check for early access
    if "Unlock Early Access chapter by signing in and purchasing" in html:
        return []

    # Check for rate limiting
    if "Rate Limited" in html:
        return []

    # Extract image URLs
    img_urls = re.findall(
        r'https://media\.qiscans\.org/file/qiscans/upload/series/[^"]*\.webp', html
    )
    # Remove /file/qiscans
    img_urls = [url.replace("/file/qiscans", "") for url in img_urls]
    # Exclude thumbnail images (case-insensitive)
    img_urls = [url for url in img_urls if "thumbnail.webp" not in url.lower()]
    img_urls = list(set(img_urls))
    img_urls.sort()

    # Skip if only 1 image (likely not a real chapter)
    if len(img_urls) <= 1:
        return []

    return img_urls


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Poster URL or None
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image with class containing "object-cover"
    poster_match = re.search(r'<img[^>]*class="[^"]*object-cover[^"]*"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Qi Scans scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://qiscans.org", timeout=30)
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
