#!/usr/bin/env python3
"""
HiveToons scraper for MAGI.

Downloads manga/manhwa/manhua from hivetoons.org.
"""

# Standard library imports
import asyncio
import os
import re
import time
from urllib.parse import urlparse

import json

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    get_scraper_config,
    get_session,
    log,
    MAX_RETRIES,
    RETRY_DELAY,
    run_scraper,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("hivetoons", "HiveToons", "[HiveToons]")
ALLOWED_DOMAINS = ["storage.hivetoon.com"]
JSON_FILE = os.getenv(
    "json_file", os.path.join(os.path.dirname(__file__), "hivetoons.json")
)
BASE_URL = "https://hivetoons.org"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from JSON cache.

    Args:
        session: requests.Session object
        page_num: Page number (only page 1 is valid for this source)

    Returns:
        tuple: (list of series info dicts with 'series_url' key, total_pages)
    """
    # hivetoons doesn't have pagination, just load from JSON on first page
    if page_num > 1:
        return [], 1

    if not os.path.exists(JSON_FILE) or os.path.getsize(JSON_FILE) == 0:
        log("Fetching all series data...")
        response = session.get(
            "https://api.hivetoons.org/api/query?page=1&perPage=99999", timeout=30
        )
        response.raise_for_status()
        with open(JSON_FILE, "w") as f:
            f.write(response.text)
    else:
        log("Loading series data from cache...")

    with open(JSON_FILE, "r") as f:
        data = json.load(f)

    series_info = []
    for post in data.get("posts", []):
        if not post.get("isNovel", True):  # Only comics, not novels
            slug = post.get("slug")
            if slug:
                series_info.append({'series_url': f"/series/{slug}"})

    return (series_info, 1)


def extract_series_title(session, series_url):
    """
    Extract series title from JSON cache.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    series_slug = series_url.replace("/series/", "")

    with open(JSON_FILE, "r") as f:
        data = json.load(f)

    for post in data.get("posts", []):
        if post.get("slug") == series_slug:
            return post.get("postTitle")

    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Poster URL or None
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image with itemprop="image"
    poster_match = re.search(r'<img[^>]*itemprop="image"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    # Fallback: Look for preload link as image
    preload_match = re.search(r'<link[^>]*rel="preload"[^>]*as="image"[^>]*href="([^"]+)"', html)
    if preload_match:
        poster_url = preload_match.group(1)
        return poster_url

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter slugs from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    series_slug = series_url.replace("/series/", "")
    full_url = f"https://hivetoons.org/series/{series_slug}"

    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", "")

    # Extract chapter slugs
    slugs = re.findall(r'\\"slug\\":\\"chapter-[^"]*\\"', html)
    chapter_slugs = []
    for slug_match in slugs:
        slug = slug_match.replace('\\"slug\\":\\"', "").replace("\\", "").rstrip('"')
        if slug not in chapter_slugs:
            chapter_slugs.append(slug)

    chapter_info = []
    for slug in chapter_slugs:
        match = re.search(r"chapter-(\d+)", slug)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': f"/series/{series_slug}/{slug}", 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs, None if locked, empty list if not found
    """
    full_url = f"https://hivetoons.org{chapter_url}"

    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(full_url, timeout=30)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text

            # Check if chapter is locked
            if "This chapter is locked" in html:
                return None  # Locked

            # Extract image URLs - try the correct JSON pattern first
            images = re.findall(
                r'"url":"(https://storage\.hivetoon\.com/public/[^"]*)"', html
            )

            # If no images found, extract from src attributes like bash script
            if not images:
                src_matches = re.findall(r'src="([^"]*)"', html)
                images = [
                    url
                    for url in src_matches
                    if urlparse(url).netloc == "storage.hivetoon.com"
                    and any(
                        url.endswith(ext)
                        for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                    )
                ]

                # If still no images, try broader patterns
                if not images:
                    images = re.findall(
                        r'https://storage\.hivetoon\.com/[^\s"]*\.(?:webp|jpg|png|jpeg|avif)',
                        html,
                    )

            # Filter out UI elements like logos - only keep images from series folders
            filtered_images = [
                url
                for url in images
                if "/upload/series/" in url and "logo" not in url.lower()
            ]

            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(filtered_images))

            if len(unique_images) >= 1:
                return unique_images
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(RETRY_DELAY)

    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting HiveToons scraper")

    # Cloudflare bypass
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies or "cf_clearance" not in cookies:
            warn("Cloudflare bypass failed, trying without bypass...")
            session = get_session()
        else:
            session = get_session(cookies, headers)
    except Exception as e:
        warn(f"Cloudflare bypass failed: {e}, trying without bypass...")
        session = get_session()

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
