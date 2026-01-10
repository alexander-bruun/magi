#!/usr/bin/env python3
"""
NexComic scraper for MAGI.

Downloads manga/manhwa/manhua from nexcomic.com.
"""

# Standard library imports
import asyncio
import re
import time
from urllib.parse import urljoin

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    error,
    get_scraper_config,
    get_session,
    log,
    MAX_RETRIES,
    run_scraper,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("nexcomic", "NexComic", "[NexComic]")
ALLOWED_DOMAINS = ["nexcomic.com", "storage.nexcomic.com"]
BASE_URL = "https://nexcomic.com"


# =============================================================================
# Utility Functions
# =============================================================================
def retry_request(url, session, max_retries=MAX_RETRIES, timeout=60):
    """
    Make a request with retry logic and exponential backoff.

    Args:
        url: URL to request
        session: requests.Session object
        max_retries: Maximum number of retry attempts
        timeout: Request timeout in seconds

    Returns:
        requests.Response object
    """
    for attempt in range(max_retries):
        try:
            response = session.get(url, timeout=timeout)
            if response.status_code == 429:
                wait_time = 2**attempt  # Exponential backoff: 1s, 2s, 4s
                warn(
                    f"Rate limited (429). Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})"
                )
                time.sleep(wait_time)
                continue
            response.raise_for_status()
            return response
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = 2**attempt
                warn(
                    f"Request failed: {e}. Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})"
                )
                time.sleep(wait_time)
            else:
                raise e


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series URLs from the manga listing page.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    log("Fetching series from manga listing page...")
    response = retry_request("https://nexcomic.com/manga/", session)
    html = response.text.replace("\n", "")

    # Extract series URLs from the listing
    series_urls = []
    # Look for links to /manga/{slug}/ - more flexible regex
    all_manga_links = re.findall(r'/manga/[^"\s\'<>]*', html)

    for link in all_manga_links:
        # Filter out non-series links
        if any(skip in link for skip in ["/feed", "/page/", "/#", "/list-mode"]):
            continue
        # Must be a series link (should have a slug after /manga/)
        if link.count("/") >= 3 and not link.endswith("/manga/"):
            if link not in series_urls:
                series_urls.append(link)

    # Return as dicts
    return [{'series_url': url} for url in series_urls]


def extract_series_title(session, series_url):
    """
    Extract series title from the series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    full_url = urljoin(BASE_URL, series_url)

    response = retry_request(full_url, session)
    html = response.text.replace("\n", "")

    # Try to extract title from various patterns
    title_patterns = [
        r"<h1[^>]*>([^<]*)</h1>",
        r"<title>([^|]*)\|",
        r'"title":"([^"]*)"',
        r'<meta property="og:title" content="([^"]*)"',
    ]

    for pattern in title_patterns:
        match = re.search(pattern, html, re.IGNORECASE)
        if match:
            title = match.group(1).strip()
            # Clean up common suffixes
            title = re.sub(r"\s*\|\s*NexComic.*$", "", title, re.IGNORECASE)
            return title

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from the series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    full_url = urljoin(BASE_URL, series_url)

    response = retry_request(full_url, session)
    html = response.text.replace("\n", "")

    # Extract chapter URLs - look for links to chapter pages
    chapter_urls = []

    # Pattern for chapter links
    chapter_patterns = [
        r'href="([^"]*chapter-[^"]*)"',
        r'href="(/[^"]*chapter[^"]*)"',
    ]

    for pattern in chapter_patterns:
        matches = re.findall(pattern, html)
        for match in matches:
            if match not in chapter_urls:
                chapter_urls.append(match)

    # Convert to dicts with url and num
    chapter_dicts = []
    for url in chapter_urls:
        match = re.search(r"chapter-(\d+)", url)
        if match:
            chapter_num = int(match.group(1))
            chapter_dicts.append({'url': url, 'num': chapter_num})
    
    # Sort by chapter number
    chapter_dicts.sort(key=lambda x: x['num'])
    
    return chapter_dicts


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: List of image URLs, None if locked, empty list if not found
    """
    full_url = urljoin(BASE_URL, chapter_url)

    for attempt in range(MAX_RETRIES):
        try:
            response = retry_request(full_url, session)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text

            # Check if chapter is locked or unavailable
            if "locked" in html.lower() or "not available" in html.lower():
                return None  # Locked/unavailable

            # Extract image URLs - try various patterns
            images = []

            # Look for img tags with src
            img_matches = re.findall(r'<img[^>]*src="([^"]*)"', html)
            for img_url in img_matches:
                if any(
                    img_url.endswith(ext)
                    for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                ):
                    # Filter out UI elements, logos, watermarks, etc.
                    skip_patterns = [
                        "logo",
                        "banner",
                        "icon",
                        "button",
                        "watermark",
                        "placeholder",
                        "loading",
                        "avatar",
                        "thumb",
                    ]
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        # Only include images that look like chapter pages (numbered files)
                        # Chapter images typically have patterns like 00.jpg, 01.png, 001.webp, etc.
                        filename = img_url.split("/")[-1].split(".")[
                            0
                        ]  # Get filename without extension
                        if re.match(r"^\d+$", filename):  # Only digits
                            images.append(img_url)

            # Also look for data-src or similar lazy loading attributes
            data_src_matches = re.findall(r'data-src="([^"]*)"', html)
            for img_url in data_src_matches:
                if any(
                    img_url.endswith(ext)
                    for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                ):
                    skip_patterns = [
                        "logo",
                        "banner",
                        "icon",
                        "button",
                        "watermark",
                        "placeholder",
                        "loading",
                        "avatar",
                        "thumb",
                    ]
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        filename = img_url.split("/")[-1].split(".")[0]
                        if re.match(r"^\d+$", filename):
                            images.append(img_url)

            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(images))

            if len(unique_images) >= 1:
                return unique_images
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(4)

    return []


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Poster URL or None
    """
    full_url = urljoin(BASE_URL, series_url)

    response = retry_request(full_url, session)
    html = response.text

    # Look for poster image with itemprop="image"
    poster_match = re.search(r'<img[^>]*itemprop="image"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting NexComic scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://nexcomic.com", timeout=30)
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
