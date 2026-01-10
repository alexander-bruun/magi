#!/usr/bin/env python3
"""
MangaKatana scraper for MAGI.

Downloads manga/manhwa/manhua from mangakatana.com.
"""

# Standard library imports
import re
import time

# Local imports
from scraper_utils import (
    error,
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
CONFIG = get_scraper_config("mangakatana", "MangaKatana", "[MangaKatana]")
ALLOWED_DOMAINS = [
    "mangakatana.com",
    "i1.mangakatana.com",
    "i2.mangakatana.com",
    "i3.mangakatana.com",
    "i4.mangakatana.com",
    "i5.mangakatana.com",
    "i6.mangakatana.com",
    "i7.mangakatana.com",
]
BASE_URL = "https://mangakatana.com"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        list: Series info dicts with 'series_url' key
    """
    if page_num == 1:
        all_series = []
        current_page = 1

        while True:
            if current_page == 1:
                url = f"{BASE_URL}/latest"
            else:
                url = f"{BASE_URL}/latest/page/{current_page}"
            log(f"Fetching series list from page {current_page}...")

            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Check if this is the last page by looking for pagination
            # MangaKatana shows "next page-numbers" link if there are more pages
            is_last_page = "next page-numbers" not in html

            # Extract series URLs - look for full manga URLs
            series_urls = re.findall(
                r'href="(https://mangakatana\.com/manga/[^"]+\.\d+)"', html
            )
            # Convert to relative URLs for consistency
            page_series = [url.replace("https://mangakatana.com", "") for url in series_urls]
            # Filter out chapter URLs (those containing /c)
            page_series = [url for url in page_series if "/c" not in url]

            # Convert to dicts
            for url in page_series:
                all_series.append({'series_url': url})

            if is_last_page:
                break
            current_page += 1

        return list(set(all_series))  # Remove duplicates
    else:
        return []


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    url = f"{BASE_URL}{series_url}"

    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract title from various possible locations
            # Try title tag first
            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = title_match.group(1).replace(" - MangaKatana", "").strip()
                return title

            # Try h1 tag
            h1_match = re.search(r"<h1[^>]*>([^<]+)</h1>", html)
            if h1_match:
                return h1_match.group(1).strip()

        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(
                    f"Failed to extract title (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract title after {MAX_RETRIES} attempts: {e}")
                return None

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

    # Look for poster image with alt="[Cover]"
    poster_match = re.search(r'<img[^>]*alt="\[Cover\]"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter links - they follow pattern https://mangakatana.com/manga/series.id/c123
    chapter_urls = re.findall(
        r'href="(https://mangakatana\.com/manga/[^"]+/c[\d.]+[^"]*)"', html
    )
    # Convert to relative URLs
    chapter_urls = [url.replace("https://mangakatana.com", "") for url in chapter_urls]

    chapter_info = []
    for url in chapter_urls:
        match = re.search(r"/c([\d.]+)", url)
        if match:
            num = float(match.group(1))
            chapter_info.append({'url': url, 'num': num})

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
        list: Image URLs
    """
    full_url = f"{BASE_URL}{chapter_url}"

    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(full_url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract image URLs from the JavaScript array thzq
            # Look for var thzq=['url1','url2',...];
            thzq_match = re.search(r"var thzq=\[([^\]]+)\];", html)
            if thzq_match:
                # Extract URLs from the array
                array_content = thzq_match.group(1)
                image_urls = re.findall(r"'(https://[^']+)'", array_content)
            else:
                # Fallback to looking for img tags
                image_urls = re.findall(
                    r'<img[^>]*src="([^"]*mangakatana\.com[^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"',
                    html,
                )

            # Filter for allowed domains and remove thumbnails
            filtered_urls = []
            for url in image_urls:
                if (
                    any(domain in url for domain in ALLOWED_DOMAINS)
                    and "thumbnail" not in url.lower()
                ):
                    filtered_urls.append(url)

            # Sort by filename number if present
            def sort_key(url):
                match = re.search(r"/(\d+)\.(jpg|jpeg|png|webp)", url)
                if match:
                    return int(match.group(1))
                return 0

            filtered_urls.sort(key=sort_key)
            return list(dict.fromkeys(filtered_urls))  # unique

        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(
                    f"Failed to extract images (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
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
    log("Starting MangaKatana scraper")

    # Create session with custom headers
    session = get_session()
    session.headers.update(
        {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
            "Accept-Language": "en-US,en;q=0.5",
            "Accept-Encoding": "gzip, deflate",
            "Connection": "keep-alive",
            "Upgrade-Insecure-Requests": "1",
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "none",
            "Cache-Control": "max-age=0",
        }
    )

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
