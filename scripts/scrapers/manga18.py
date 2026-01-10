#!/usr/bin/env python3
"""
Manga18 scraper for MAGI.

Downloads manga/manhwa/manhua from manga18.me.
"""

# Standard library imports
import re
import time

import json

# Third-party imports
import requests

# Local imports
from scraper_utils import (
    get_scraper_config,
    log,
    MAX_RETRIES,
    RETRY_DELAY,
    run_scraper,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("manga18", "Manga18", "[Manga18]")
ALLOWED_DOMAINS = ["manga18.me", "manga18.com"]
BASE_URL = "https://manga18.me"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page with pagination.

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
                url = f"{BASE_URL}/manga/"
            else:
                url = f"{BASE_URL}/manga/{current_page}/"

            response = session.get(url, timeout=30)

            # Check if page exists (404 means no more pages)
            if response.status_code == 404:
                break

            response.raise_for_status()
            html = response.text

            # Extract series URLs (both absolute and relative)
            series_urls = re.findall(r'href="([^"]*manga/[^"]*)/?"', html)

            # Convert relative URLs to absolute and filter
            page_series = []
            for url in series_urls:
                if url.startswith("/"):
                    url = f"{BASE_URL}{url}"
                if (
                    url.startswith(f"{BASE_URL}/manga/")
                    and "/chapter-" not in url
                    and not url.endswith("/manga/")
                    and not url.endswith("/manga")
                    and not re.search(r"/manga/\d+/?$", url)
                ):  # Exclude numeric-only URLs like /manga/2
                    relative_url = url.replace(BASE_URL, "")
                    page_series.append({'series_url': relative_url})

            if not page_series:
                break

            all_series.extend(page_series)
            current_page += 1

        return list(set(all_series))  # Remove duplicates
    else:
        return []


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if not found
    """
    for attempt in range(MAX_RETRIES):
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
                title = title_match.group(1).replace(" &#8211; Manga18", "").strip()
                if title:
                    return title
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(
                    f"Failed to extract title (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)

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

    # Look for poster image from allowed domains
    for domain in ALLOWED_DOMAINS:
        poster_match = re.search(rf'<img[^>]*src="([^"]*{re.escape(domain)}[^"]*)"', html)
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

    # Extract chapter URLs (both absolute and relative)
    chapter_urls = re.findall(r'href="([^"]*chapter-[^"]*)/?"', html)

    chapter_info = []
    for url in set(chapter_urls):
        if url.startswith("/"):
            url = f"{BASE_URL}{url}"
        if url.startswith(full_url) and "chapter-" in url:
            match = re.search(r"chapter-(\d+)", url)
            if match:
                num = int(match.group(1))
                relative_url = url.replace(BASE_URL, "")
                chapter_info.append({'url': relative_url, 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", " ")

    # Extract image URLs from chapter_preloaded_images script
    script_match = re.search(r"chapter_preloaded_images = (\[[\s\S]*?\])", html)
    if script_match:
        try:
            image_urls = json.loads(script_match.group(1))
            # Remove duplicates while preserving order
            return list(dict.fromkeys(image_urls))
        except json.JSONDecodeError:
            pass

    # Fallback: Extract image URLs from data-src attributes
    image_urls = re.findall(
        r'data-src=[\'"](https?://img\d*\.manga18\.(?:me|com)/[^\'"]*\.(?:jpg|jpeg|png|webp))[\'"]',
        html,
    )
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))

    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(
            r'src=[\'"](https?://img\d*\.manga18\.(?:me|com)/[^\'"]*\.(?:jpg|jpeg|png|webp))[\'"]',
            html,
        )
        image_urls = list(dict.fromkeys(image_urls))

    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Manga18 scraper")

    # Health check (no Cloudflare bypass needed)
    session = requests.Session()

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
