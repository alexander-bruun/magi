#!/usr/bin/env python3
"""
Asura Scans scraper for MAGI.

Downloads manga/manhwa/manhua from asuracomic.net.
"""

# Standard library imports
import re
import time

# Local imports
from scraper_utils import (
    MAX_RETRIES,
    RETRY_DELAY,
    get_scraper_config,
    get_session,
    log,
    run_scraper,
    warn,
    error,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("asurascans", "AsuraScans", "[AsuraScans]")
ALLOWED_DOMAINS = ["gg.asuracomic.net"]
BASE_URL = "https://asuracomic.net"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series data from listing page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, total_pages)
    """
    if page_num > 1:
        return [], 1  # Only fetch on first page

    all_series_urls = []
    current_page = 1
    max_pages = 100  # Safety limit
    while current_page <= max_pages:
        url = f"{BASE_URL}/series?page={current_page}"
        log(f"Fetching series list from page {current_page}...")

        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if this is the last page by looking for disabled "Next" button
        is_last_page = "pointer-events: none" in html and "Next" in html

        # Match series URLs with the 8-character hex suffix pattern (e.g., series/nano-machine-be19545a)
        series_urls = re.findall(r'href="series/[a-z0-9-]+-[a-f0-9]{8}"', html)
        # Remove href=" and add leading /
        page_series_urls = [url.replace('href="', "/").rstrip('"') for url in series_urls]
        all_series_urls.extend(page_series_urls)
        log(f"Found {len(page_series_urls)} series on page {current_page}")

        if is_last_page or not page_series_urls:
            log(f"Reached last page or no more series (page {current_page}).")
            break
        current_page += 1

    # Remove duplicates
    all_series_urls = sorted(set(all_series_urls))
    log(f"Found {len(all_series_urls)} total series")

    # Convert to dicts with series_slug
    series_data = [{'series_slug': url.split('/series/')[-1]} for url in all_series_urls]
    return series_data, current_page


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL path of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract title from <title> tag and remove " - Asura Scans" suffix
            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = title_match.group(1).replace(" - Asura Scans", "").strip()
                return title
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


def extract_poster_url(session, series_url):
    """
    Extract series poster URL from series page like: https://gg.asuracomic.net/storage/media/380081/conversions/01K9J70BH0FKMQXE745SXBR25K-optimized.webp.

    Args:
        session: requests.Session object
        series_url: Relative URL path of the series
    Returns:
        str: Poster image URL, or None if extraction failed
    """
    url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Look for poster image URL in the HTML - try multiple patterns
            # First try: img tag with alt="poster" and src containing optimized.webp
            poster_match = re.search(r'<img[^>]*alt="poster"[^>]*src="([^"]*optimized\.webp[^"]*)"', html)
            if poster_match:
                poster_url = poster_match.group(1)
            else:
                # Second try: any URL containing optimized.webp (fallback)
                poster_match = re.search(r'https://[^"]*optimized\.webp', html)
                if poster_match:
                    poster_url = poster_match.group(0)
                else:
                    # Third try: look for poster or cover images
                    poster_match = re.search(r'(https://gg\.asuracomic\.net/storage/media/\d+/conversions/[^"]*optimized\.webp)', html)
                    if poster_match:
                        poster_url = poster_match.group(1)
                    else:
                        return None

            # Make sure it's absolute URL
            if poster_url.startswith('//'):
                poster_url = 'https:' + poster_url
            elif poster_url.startswith('/'):
                poster_url = BASE_URL.rstrip('/') + poster_url
            return poster_url
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract poster (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract poster after {MAX_RETRIES} attempts: {e}")
                return None

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL path of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    series_slug = series_url.split("/series/")[-1]
    # Extract chapter links like series_slug/chapter/123 or series_slug/chapter/1.2
    chapter_patterns = re.findall(rf"{re.escape(series_slug)}/chapter/\d+(?:\.\d+)?", html)
    # Convert to full URLs
    chapter_urls = [f"/series/{pattern}" for pattern in chapter_patterns]
    # Sort by chapter number
    chapter_urls.sort(key=lambda x: float(re.search(r"/chapter/([\d.]+)", x).group(1)))
    chapter_urls = list(dict.fromkeys(chapter_urls))  # unique

    chapter_info = []
    for url in chapter_urls:
        match = re.search(r"/chapter/([\d.]+)", url)
        if match:
            num = float(match.group(1))
            chapter_info.append({'url': url, 'num': num})

    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL path of the chapter

    Returns:
        list: Image URLs in reading order, empty list if unavailable
    """
    full_url = f"{BASE_URL}{chapter_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract image URLs from JSON data embedded in the page
            # The page contains escaped JSON with "order" and "url" fields
            # Try general pattern first
            pattern = r'\\"order\\":\d+,\\"url\\":\\"https://[^"\\]+\.(?:jpg|webp)\\"'
            matches = re.findall(pattern, html)

            if matches:
                # Extract URLs and sort by order
                urls_with_order = []
                for match in matches:
                    # Extract order and url
                    order_match = re.search(r'\\"order\\":(\d+)', match)
                    url_match = re.search(r'\\"url\\":\\"([^"\\]+)\\"', match)
                    if order_match and url_match:
                        order = int(order_match.group(1))
                        url = url_match.group(1)
                        urls_with_order.append((order, url))

                # Sort by order and extract URLs
                urls_with_order.sort(key=lambda x: x[0])
                image_urls = [url for _, url in urls_with_order]
                # Remove duplicates while preserving order
                seen = set()
                image_urls = [x for x in image_urls if not (x in seen or seen.add(x))]
                return image_urls
        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract images (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract images after {MAX_RETRIES} attempts: {e}")
                return []

    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Asura Scans scraper")
    log("Mode: Full Downloader")

    # Create session
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
        series_url_builder=lambda data: f"/series/{data.get('series_slug')}"
    )


if __name__ == "__main__":
    main()
