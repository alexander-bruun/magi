#!/usr/bin/env python3
"""
ToonGod scraper for MAGI.

Downloads manga/manhwa/manhua from www.toongod.org.
"""

# Standard library imports
import asyncio
import re
from urllib.parse import urlparse

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
CONFIG = get_scraper_config("toongod", "ToonGod", "[ToonGod]")
ALLOWED_DOMAINS = ["www.toongod.org", "i.tngcdn.com"]
BASE_URL = "https://www.toongod.org"


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
        if page == 1:
            url = "https://www.toongod.org/home/"
        else:
            url = f"https://www.toongod.org/home/page/{page}/"
        log(f"Fetching series list from page {page}...")

        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if this is the last page by looking for "next" link
        is_last_page = "next page-numbers" not in html and "Next" not in html

        # Extract series URLs - look for webtoon entry links
        series_urls = re.findall(
            r'href="https://www\.toongod\.org(/webtoon/[^/"]*/)"', html
        )
        # Filter out chapter URLs and other non-series URLs
        series_urls = [
            url
            for url in series_urls
            if "chapter" not in url and "feed" not in url and "genre" not in url
        ]

        # Convert to dict format
        for series_url in series_urls:
            all_series_urls.append({'series_url': series_url})

        if is_last_page or not series_urls:
            break
        page += 1

    return sorted(set(all_series_urls), key=lambda x: x['series_url'])


def extract_series_title(session, series_url):
    """
    Extract series title from the series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    title_match = re.search(r"<title>([^<]+)", html)
    if title_match:
        title = (
            title_match.group(1)
            .replace(" Manhwa ToonGod", "")
            .replace(" | ToonGod", "")
            .strip()
        )
        return title
    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Poster URL, or None if not found
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
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
def extract_chapter_urls(session, manga_url):
    """
    Extract chapter URLs for a given manga.

    Args:
        session: requests.Session object
        manga_url: Relative URL of the manga

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    full_url = f"{BASE_URL}{manga_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\0", "")

    chapter_urls = re.findall(
        r'href="https://www\.toongod\.org(/webtoon/[^/]+/chapter-[^/"]*/)"', html
    )

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
    Extract image URLs for a given chapter.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: List of image URLs
    """
    full_url = f"{BASE_URL}{chapter_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\0", "")

    # Look for img src attributes that contain manga images
    image_urls = re.findall(r'<img[^>]*src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
    # Also check for data-src (lazy loading)
    data_src_urls = re.findall(
        r'<img[^>]*data-src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html
    )
    image_urls.extend(data_src_urls)
    # Clean URLs and filter for toongod images
    cleaned_urls = []
    for url in image_urls:
        url = url.strip()
        parsed = urlparse(url)
        if (
            parsed.scheme in ("http", "https")
            and parsed.netloc in ("i.tngcdn.com", "toongod.org")
            and "wp-content" not in parsed.path
            and "logo" not in parsed.path
            and "assets" not in parsed.path
        ):
            cleaned_urls.append(url)
    return list(dict.fromkeys(cleaned_urls))  # unique


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting ToonGod scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://www.toongod.org", timeout=30)
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
