#!/usr/bin/env python3
"""
ManhwaGalaxy scraper for MAGI.

Downloads manga/manhwa from manhwagalaxy.com.
"""

# Standard library imports
import re
import time

# Third-party imports
import requests

# Local imports
from scraper_utils import (
    error,
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
CONFIG = get_scraper_config("manhwagalaxy", "ManhwaGalaxy", "[ManhwaGalaxy]")
ALLOWED_DOMAINS = ["manhwagalaxy.com"]
BASE_URL = "https://manhwagalaxy.com"


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
        url = f"https://manhwagalaxy.com/page/{page}/"
        response = session.get(url, timeout=30)

        # Check if page exists (404 means no more pages)
        if response.status_code == 404:
            break

        response.raise_for_status()
        html = response.text

        # Extract series URLs (both absolute and relative)
        series_urls = re.findall(r'href="([^"]*manhwa/[^"]*)/?"', html)

        # Convert relative URLs to absolute and filter
        processed_urls = []
        for url in series_urls:
            if url.startswith("/"):
                url = f"https://manhwagalaxy.com{url}"
            if (
                url.startswith("https://manhwagalaxy.com/manhwa/")
                and "/chapter-" not in url
                and not url.endswith("/manhwa/")
                and not url.endswith("/manhwa")
            ):
                processed_urls.append(url)

        if not processed_urls:
            break
            
        # Add to all series as dicts
        for url in processed_urls:
            all_series_urls.append({'series_url': url})
            
        page += 1
    
    return sorted(set(all_series_urls), key=lambda x: x['series_url'])


# Extract series title from series page
def extract_series_title(session, series_url):
    """
    Extract series title from the series page.

    Args:
        session: requests.Session object
        series_url: URL of the series page

    Returns:
        str: Series title, or None if not found
    """
    for i in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Try to extract from h1 tag first (actual manga title)
            h1_match = re.search(r"<h1[^>]*>([^<]+)</h1>", html)
            if h1_match:
                title = h1_match.group(1).strip()
                if title:
                    return title

            # Try to extract from span with title class
            span_match = re.search(
                r'<span[^>]*class="[^"]*title[^"]*"[^>]*>([^<]+)</span>',
                html,
                re.IGNORECASE,
            )
            if span_match:
                title = span_match.group(1).strip()
                if title:
                    return title

            # Fallback to title tag
            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = (
                    title_match.group(1).replace(" &#8211; ManhwaGalaxy", "").strip()
                )
                if title:
                    return title
        except Exception as e:
            if i < MAX_RETRIES:
                warn(
                    f"Failed to extract title (attempt {i}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from the series page.

    Args:
        session: requests.Session object
        series_url: URL of the series page

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter URLs (both absolute and relative)
    chapter_urls = re.findall(r'href="([^"]*chapter-[^"]*)/?"', html)

    # Convert relative URLs to absolute
    processed_urls = []
    for url in chapter_urls:
        if url.startswith("/"):
            url = f"https://manhwagalaxy.com{url}"
        if url.startswith("https://manhwagalaxy.com/manhwa/") and "chapter-" in url:
            processed_urls.append(url)

    # Convert to dicts with url and num
    chapter_dicts = []
    for url in processed_urls:
        # Extract chapter number from URL
        chapter_match = re.search(r'chapter-(\d+)', url)
        if chapter_match:
            chapter_num = int(chapter_match.group(1))
            chapter_dicts.append({'url': url, 'num': chapter_num})
    
    # Remove duplicates and sort by chapter number
    unique_dicts = []
    seen_urls = set()
    for chapter in sorted(chapter_dicts, key=lambda x: x['num']):
        if chapter['url'] not in seen_urls:
            unique_dicts.append(chapter)
            seen_urls.add(chapter['url'])
    
    return unique_dicts


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: URL of the chapter page

    Returns:
        list: List of image URLs
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", " ")

    # Extract image URLs from data-src attributes
    image_urls = re.findall(
        r'data-src=[\'"](https?://img-\d*\.manhwagalaxy\.com/[^\'"]*\.(?:jpg|jpeg|png|webp))[\'"]',
        html,
    )
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))

    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(
            r'src=[\'"](https?://img-\d*\.manhwagalaxy\.com/[^\'"]*\.(?:jpg|jpeg|png|webp))[\'"]',
            html,
        )
        image_urls = list(dict.fromkeys(image_urls))

    return image_urls


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL of the series page

    Returns:
        str: Poster URL or None
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image with class="img-loading"
    poster_match = re.search(r'<img[^>]*class="img-loading[^"]*"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting ManhwaGalaxy scraper")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        session = requests.Session()
        response = session.get("https://manhwagalaxy.com", timeout=30)
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
