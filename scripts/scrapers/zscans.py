#!/usr/bin/env python3
"""
ZScans scraper for MAGI.

Downloads manga/manhwa/manhua from zscans.com.
"""

# Standard library imports
import asyncio
import re
import time

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    error,
    get_scraper_config,
    get_session,
    log,
    MAX_RETRIES,
    run_scraper,
    RETRY_DELAY,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("zscans", "ZScans", "[ZScans]")
ALLOWED_DOMAINS = ["zscans.com"]
BASE_URL = "https://zscans.com"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series slugs from the comics page.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    url = "https://zscans.com/comics"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract series slugs from quoted strings, filter out common terms
    slugs = re.findall(r'"([a-z0-9-]+)"', html)
    # Filter out common HTML/CSS terms
    exclude_terms = {
        "css",
        "js",
        "png",
        "jpg",
        "webp",
        "svg",
        "ico",
        "woff",
        "ttf",
        "eot",
        "px",
        "app",
        "button",
        "alert",
        "all",
        "canonical",
        "charset",
        "content",
        "data",
        "div",
        "form",
        "head",
        "html",
        "http",
        "icon",
        "img",
        "input",
        "link",
        "meta",
        "nav",
        "none",
        "page",
        "path",
        "rel",
        "script",
        "span",
        "style",
        "text",
        "title",
        "type",
        "url",
        "var",
        "view",
        "xml",
        "lang",
        "language",
        "container",
        "bookmark",
        "horizontal",
        "font-weight-bold",
        "hooper-list",
        "hooper-next",
        "hooper-prev",
        "hooper-track",
        "action",
        "comedy",
        "drama",
        "fantasy",
        "horror",
        "isekai",
        "manga",
        "manhua",
        "manhwa",
        "mystery",
        "romance",
        "supernatural",
        "historical",
        "completed",
        "dropped",
        "ongoing",
        "hiatus",
        "new",
        "one-shot",
        "martial-arts",
        "reincarnation",
    }

    series_info = []
    for slug in slugs:
        if slug not in exclude_terms and re.match(
            r"^[a-z][a-z0-9]*(-[a-z0-9]+)+$", slug
        ):
            series_info.append({'series_url': f"/comics/{slug}"})

    return sorted(set(series_info), key=lambda x: x['series_url'])


# Extract series title from series page
def extract_series_title(session, series_url):
    """Extract the series title from a series page.

    Args:
        session: requests.Session object with authentication
        series_url: URL of the series

    Returns:
        str: The series title, or None if extraction failed
    """
    # Extract series_slug from URL
    match = re.search(r'/comics/([^/]+)', series_url)
    if not match:
        return None
    
    series_slug = match.group(1)
    
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            url = f"{BASE_URL}/comics/{series_slug}"
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Try to extract title from JavaScript data first
            title_match = re.search(r'name:"([^"]*)"', html)
            if title_match:
                return title_match.group(1)

            # Fallback to page title
            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = (
                    title_match.group(1)
                    .replace(" • Zero Scans", "")
                    .replace("Read ", "")
                    .replace(" with up to date chapters!", "")
                )
                return title.strip()
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
        series_url: URL of the series

    Returns:
        str: Poster URL, or None if not found
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image in the series page
    poster_match = re.search(r'<img[^>]*src="([^"]*poster[^"]*)"', html, re.IGNORECASE)
    if poster_match:
        return poster_match.group(1)

    # Fallback: look for any large image that might be the poster
    img_match = re.search(r'<img[^>]*src="([^"]*\.(?:jpg|png|webp))[^"]*"[^>]*class="[^"]*poster[^"]*"', html, re.IGNORECASE)
    if img_match:
        return img_match.group(1)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================


def extract_chapter_urls(session, series_url):
    """Extract chapter URLs from a series page.

    Args:
        session: requests.Session object with authentication
        series_url: URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    # Extract series_slug from URL
    match = re.search(r'/comics/([^/]+)', series_url)
    if not match:
        return []
    
    series_slug = match.group(1)
    
    url = f"{BASE_URL}/comics/{series_slug}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter count and first chapter ID
    chapter_count_match = re.search(r"chapters_count:(\d+)", html)
    first_chapter_match = re.search(r"first_chapter:\[\{[^}]*?,id:(\d+)\}", html)

    if not chapter_count_match or not first_chapter_match:
        error(f"Could not find chapter information in {url}")
        return []

    chapter_count = int(chapter_count_match.group(1))
    first_chapter_id = int(first_chapter_match.group(1))

    # Generate chapter URLs assuming sequential IDs
    chapter_info = []
    for i in range(chapter_count):
        chapter_id = first_chapter_id + i
        chapter_url = f"https://zscans.com/comics/{series_slug}/{chapter_id}"
        chapter_info.append({'url': chapter_url, 'num': chapter_id})

    return chapter_info


def extract_image_urls(session, chapter_url):
    """Extract image URLs from a chapter page.

    Args:
        session: requests.Session object with authentication
        chapter_url: Full URL to the chapter page

    Returns:
        list: List of image URLs
    """
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(chapter_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract image URLs from JavaScript data
            # Look for high_quality or good_quality arrays
            images_match = re.search(r"(high_quality|good_quality):\[(.*?)\]", html)
            if images_match:
                images_data = images_match.group(2)
                # Extract URLs and unescape \u002F to /
                urls = re.findall(r'"([^"]*)"', images_data)
                image_urls = []
                for url in urls:
                    url = url.replace("\\u002F", "/")
                    if url.startswith("https://"):
                        image_urls.append(url)
                return image_urls
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
    log("Starting Z Scans scraper")

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
