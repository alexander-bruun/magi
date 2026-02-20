#!/usr/bin/env python3
"""
Demonic Scans scraper for MAGI.

Downloads manga/manhwa/manhua from demonicscans.org.
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
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("demonicscans", "DemonicScans", "[DemonicScans]")
ALLOWED_DOMAINS = [
    "demoniclibs.com",
    "mangareadon.org",
    "readermc.org",
    "mangafirst.org",
    "mangathird.org",
    "mangasecond.org",
    "mangafourth.org",
    "mangafifth.org",
]
BASE_URL = "https://demonicscans.org"


# =============================================================================
# Helper Functions
# =============================================================================
def make_request_with_bypass_retry(session, url, timeout=30):
    """
    Make a GET request with automatic Cloudflare bypass retry on 403 errors.
    
    Args:
        session: requests.Session object
        url: URL to request
        timeout: Request timeout in seconds
        
    Returns:
        requests.Response: The response object
        
    Raises:
        Exception: If request fails after retry
    """
    try:
        response = session.get(url, timeout=timeout)
        response.raise_for_status()
        return response
    except Exception as e:
        if hasattr(e, 'response') and e.response and e.response.status_code == 403:
            log(f"403 error encountered for {url}, attempting re-bypass...")
            try:
                # Re-bypass Cloudflare
                cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
                if cookies and headers:
                    # Update session with new cookies and headers
                    session.cookies.update(cookies)
                    session.headers.update(headers)
                    log("Re-bypass successful, retrying request...")
                    # Retry the request
                    response = session.get(url, timeout=timeout)
                    response.raise_for_status()
                    return response
                else:
                    error("Re-bypass failed")
                    raise e
            except Exception as retry_e:
                error(f"Re-bypass retry failed: {retry_e}")
                raise e
        else:
            raise e


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
        url = f"{BASE_URL}/lastupdates.php?list={current_page}"
        log(f"Fetching series list from page {current_page}...")

        response = make_request_with_bypass_retry(session, url, timeout=30)
        html = response.text

        # Check if this is the last page by looking for disabled "Next" button
        is_last_page = "pointer-events: none" in html and "Next" in html

        # Extract series URLs
        series_urls = re.findall(r'href="(/manga/[^"]+)"', html)
        all_series_urls.extend(series_urls)
        log(f"Found {len(series_urls)} series on page {current_page}")

        if is_last_page or not series_urls:
            log(f"Reached last page or no more series (page {current_page}).")
            break
        current_page += 1

    # Remove duplicates
    all_series_urls = sorted(set(all_series_urls))
    log(f"Found {len(all_series_urls)} total series")

    # Convert to dicts with series_url
    series_data = [{'series_url': url} for url in all_series_urls]
    return series_data, current_page


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
    try:
        response = make_request_with_bypass_retry(session, url, timeout=30)
    except Exception as e:
        error(f"Failed to fetch series page {url}: {e}")
        return None
    html = response.text

    title_match = re.search(r"<title>([^<]+)", html)
    if title_match:
        title = title_match.group(1).replace(" - Demonic Scans", "").strip()
        # Replace underscores with spaces (they may come from the HTML)
        title = title.replace("_", " ")
        return title
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
    response = make_request_with_bypass_retry(session, url, timeout=30)
    html = response.text

    # Look for poster image with class="border-box"
    poster_match = re.search(r'<img[^>]*class="border-box"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, manga_url):
    """
    Extract chapter URLs for a given manga.

    Args:
        session: requests.Session object
        manga_url: Relative URL of the manga

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    full_url = f"{BASE_URL}{manga_url}"
    response = make_request_with_bypass_retry(session, full_url, timeout=30)
    html = response.text.replace("\0", "")

    chapter_urls = re.findall(
        r'href="(/chaptered\.php\?manga=[^&]+&chapter=[^"]+)"', html
    )
    
    chapter_info = []
    for url in sorted(set(chapter_urls)):
        match = re.search(r"chapter=(\d+)", url)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': url, 'num': num})
    
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs for a given chapter.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs in reading order
    """
    full_url = f"{BASE_URL}{chapter_url}"
    response = make_request_with_bypass_retry(session, full_url, timeout=30)
    html = response.text.replace("\0", "")

    # Find all img tags
    img_tags = re.findall(r'<img[^>]*>', html)
    image_urls = []
    for tag in img_tags:
        # Skip if data-tried="true" is present
        if 'data-tried="true"' in tag:
            continue
        # Extract src or data-src attribute
        src_match = re.search(r'(?:data-src|src)="([^"]+)"', tag)
        if src_match:
            url = src_match.group(1)
            if any(domain in url for domain in ALLOWED_DOMAINS) and "thumbnails" not in url:
                image_urls.append(url)
    
    return list(dict.fromkeys(image_urls))  # unique


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Demonic Scans scraper")
    log("Mode: Full Downloader")

    # Health check
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
        series_url_builder=lambda data: data['series_url']
    )


if __name__ == "__main__":
    main()
