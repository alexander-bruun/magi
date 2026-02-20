#!/usr/bin/env python3
"""
Kunmanga scraper for MAGI.

Downloads manga/manhwa/manhua from kunmanga.com.
"""

# Standard library imports
import asyncio
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
CONFIG = get_scraper_config("kunmanga", "Kunmanga", "[Kunmanga]")
ALLOWED_DOMAINS = ["kunmanga.com", "kunsv1.com", "manimg24.com"]
BASE_URL = "https://kunmanga.com"


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
        # Fetch all pages on first call
        all_series = []
        current_page = 1
        while True:
            url = f"{BASE_URL}/manga/page/{current_page}/?m_orderby=new-manga"
            response = session.get(url, timeout=30)

            # Check if page exists (404 means no more pages)
            if response.status_code == 404:
                break

            response.raise_for_status()
            html = response.text

            # Extract series URLs
            series_urls = re.findall(r'href="https://kunmanga\.com/manga/[^"]*/"', html)
            # Remove href=" and " and filter out chapter and feed URLs
            page_series = [
                url.replace('href="', "").rstrip('"')
                for url in series_urls
                if "/chapter-" not in url
                and "/ch-" not in url
                and "/feed/" not in url
                and not url.endswith("/page/")
            ]

            if not page_series:
                break

            # Convert to relative URLs and add to all_series
            for url in page_series:
                relative_url = url.replace(BASE_URL, "")
                all_series.append({'series_url': relative_url})

            current_page += 1

        # Remove duplicates based on series_url
        seen = set()
        unique_series = []
        for series in all_series:
            series_url = series['series_url']
            if series_url not in seen:
                seen.add(series_url)
                unique_series.append(series)

        return unique_series, 1  # All fetched in one page
    else:
        return [], 1


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

            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = title_match.group(1).replace(" &#8211; Kunmanga", "").strip()
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
    Extract chapter URLs from series page via AJAX.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    ajax_url = f"{BASE_URL}{series_url}ajax/chapters/"
    response = session.post(ajax_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter URLs
    chapter_urls = re.findall(
        r'href="https://kunmanga\.com/manga/[^"]*chapter-[^"]*/"', html
    )
    chapter_urls = [url.replace('href="', "").rstrip('"') for url in chapter_urls]

    chapter_info = []
    for url in set(chapter_urls):
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
            # Filter to only include chapter images (kunsv1.com or manimg24.com)
            image_urls = [
                url
                for url in image_urls
                if urlparse(url).netloc.endswith(("kunsv1.com", "manimg24.com"))
            ]
            # Remove duplicates while preserving order
            return list(dict.fromkeys(image_urls))
        except json.JSONDecodeError:
            pass

    # Fallback: Extract image URLs from data-src attributes (multiple domains)
    image_urls = re.findall(
        r'data-src="\s*(https?://(?:kunmanga\.com|sv\d*\.kunsv1\.com|h\d*\.manimg24\.com)/[^"]*\.(?:jpg|jpeg|png|webp))',
        html,
    )
    # Filter to only include chapter images (kunsv1.com or manimg24.com)
    image_urls = [
        url
        for url in image_urls
        if urlparse(url).netloc.endswith(("kunsv1.com", "manimg24.com"))
    ]
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))

    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(
            r'src="\s*(https?://(?:kunmanga\.com|sv\d*\.kunsv1\.com|h\d*\.manimg24\.com)/[^"]*\.(?:jpg|jpeg|png|webp))',
            html,
        )
        # Filter to only include chapter images (kunsv1.com or manimg24.com)
        image_urls = [
            url
            for url in image_urls
            if urlparse(url).netloc.endswith(("kunsv1.com", "manimg24.com"))
        ]
        image_urls = list(dict.fromkeys(image_urls))

    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Kunmanga scraper")

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

