#!/usr/bin/env python3
"""
DrakeComic scraper for MAGI.

Downloads manga/manhwa/manhua from drakecomic.org.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import sys
import time
from pathlib import Path

import requests

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    get_default_headers,
    get_scraper_config,
    get_session,
    log,
    log_scraper_summary,
    process_chapter,
    process_series,
    run_scraper,
    success,
    warn,
    error,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("drakecomic", "DrakeComic", "[DrakeComic]")
ALLOWED_DOMAINS = ["drakecomic.org"]
BASE_URL = "https://drakecomic.org"


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
        url = f"{BASE_URL}/manga/?page={current_page}&status=&type=&order="

        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract series URLs from the page
            # Look for links to individual manga pages
            series_urls = re.findall(r'href="(https://drakecomic\.org/manga/[^"]*/)"', html)
            all_series_urls.extend(series_urls)
            log(f"Found {len(series_urls)} series on page {current_page}")

            # Check if there's a next page
            has_next_page = (
                "page=" + str(current_page + 1) in html or f"?page={current_page + 1}" in html
            )

            if not has_next_page:
                log(f"Reached last page or no more series (page {current_page}).")
                break
            current_page += 1

        except Exception as e:
            error(f"Error extracting series from page {current_page}: {e}")
            break

    # Remove duplicates
    all_series_urls = list(set(all_series_urls))
    log(f"Found {len(all_series_urls)} total series")

    # Convert to dicts with series_url
    series_data = [{'series_url': url} for url in all_series_urls]
    return series_data, current_page


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Series title, or None if extraction failed
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Look for the title in various possible locations
        title_match = re.search(r"<h1[^>]*>([^<]+)</h1>", html)
        if title_match:
            return title_match.group(1).strip()

        # Try other patterns
        title_match = re.search(r"<title>([^|]+)", html)
        if title_match:
            return title_match.group(1).strip()

        return None

    except Exception as e:
        error(f"Error extracting title from {series_url}: {e}")
        return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        str: Poster URL or None
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Look for poster image
        poster_match = re.search(r'<img[^>]*src="([^"]*drakecomic\.org[^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
        if poster_match:
            poster_url = poster_match.group(1)
            return poster_url

        return None

    except Exception as e:
        error(f"Error extracting poster from {series_url}: {e}")
        return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter links from series page.

    Args:
        session: requests.Session object
        series_url: Full URL of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text.replace("\n", "")

        # Extract chapter URLs - looking for links to chapter pages
        chapter_urls = re.findall(
            r'href="(https://drakecomic\.org/[^"]*chapter-\d+[^"]*)"', html
        )

        # Remove duplicates
        chapter_urls = list(set(chapter_urls))

        chapter_info = []
        for url in chapter_urls:
            match = re.search(r"chapter-(\d+)", url)
            if match:
                num = int(match.group(1))
                chapter_info.append({'url': url, 'num': num})

        # Sort by chapter number
        chapter_info.sort(key=lambda x: x['num'])

        return chapter_info

    except Exception as e:
        error(f"Error extracting chapters from {series_url}: {e}")
        return []


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs in reading order, None if locked, empty list if no images
    """
    full_url = chapter_url

    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(full_url, timeout=30)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text

            # Check if chapter is locked or premium - be more specific
            locked_indicators = [
                "this chapter is locked",
                "chapter locked",
                "premium content only",
                "login required to view",
                "please login to read",
                "members only content",
            ]
            is_locked = any(
                indicator in html.lower() for indicator in locked_indicators
            )

            # Extract image URLs - try multiple patterns
            images = []

            # Pattern 1: src attributes
            src_images = re.findall(
                r'src="(https://drakecomic\.org/wp-content/uploads/[^"]*\.(?:webp|jpg|png|jpeg|avif))"',
                html,
            )
            images.extend(src_images)

            # Pattern 2: direct URLs in HTML
            direct_images = re.findall(
                r'https://drakecomic\.org/wp-content/uploads/[^\s"<>\']*\.(?:webp|jpg|png|jpeg|avif)',
                html,
            )
            images.extend(direct_images)

            # Pattern 3: data-src attributes (lazy loading)
            data_src_images = re.findall(
                r'data-src="(https://drakecomic\.org/wp-content/uploads/[^"]*\.(?:webp|jpg|png|jpeg|avif))"',
                html,
            )
            images.extend(data_src_images)

            # Pattern 4: data-url attributes
            data_url_images = re.findall(
                r'data-url="(https://drakecomic\.org/wp-content/uploads/[^"]*\.(?:webp|jpg|png|jpeg|avif))"',
                html,
            )
            images.extend(data_url_images)

            # Pattern 5: JavaScript sources array (most important for this site)
            # Look for the images array in ts_reader.run() JavaScript
            js_match = re.search(r'"images":\s*\[([^\]]+)\]', html, re.DOTALL)
            if js_match:
                images_block = js_match.group(1)
                # Extract individual image URLs from the images array (handles escaped URLs)
                js_images = re.findall(
                    r'"(https:\\\/\\\/drakecomic\.org\\\/wp-content\\\/uploads[^"]*\.(?:webp|jpg|png|jpeg|avif))"',
                    images_block,
                )
                # Unescape the URLs
                unescaped_images = [url.replace("\\/", "/") for url in js_images]
                images.extend(unescaped_images)

            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(images))

            # Filter out small images, logos, etc.
            filtered_images = []
            for url in unique_images:
                # Skip if it's clearly a UI element or thumbnail
                skip_keywords = [
                    "logo",
                    "icon",
                    "button",
                    "avatar",
                    "thumbnail",
                    "thumb",
                    "cropped",
                    "small",
                    "favicon",
                ]
                if any(skip_word in url.lower() for skip_word in skip_keywords):
                    continue

                # Skip images with dimension specifications in filename (e.g., -227x300, -32x32)
                filename = url.split("/")[-1]
                if re.search(r"-\d+x\d+", filename):
                    continue

                # Must be from wp-content/uploads
                if "/wp-content/uploads/" in url:
                    filtered_images.append(url)

            # If we found images, the chapter is not locked
            if filtered_images:
                is_locked = False

            if is_locked:
                log(f"Chapter appears locked (no images found)")
                return None  # Locked

            # If we found images, the chapter is not locked
            if filtered_images:
                is_locked = False

            if is_locked:
                log(f"Chapter appears locked (no valid images found)")
                return None  # Locked

            if len(filtered_images) >= 1:
                return filtered_images
            else:
                log(f"No valid comic images found in chapter {chapter_url}")
                return []

        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(RETRY_DELAY)

    return []


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting DrakeComic scraper")
    log("Mode: Full Downloader")

    # Health check
    log(f"Performing health check on {BASE_URL}...")
    try:
        # Try without Cloudflare bypass first
        session = get_session(headers=get_default_headers())

        response = session.get(BASE_URL, timeout=30)
        if response.status_code != 200:
            log("Direct access failed, trying Cloudflare bypass...")
            cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
            if not cookies:
                return
            session = get_session(cookies, headers)
            response = session.get(BASE_URL, timeout=30)
            if response.status_code != 200:
                error(f"Health check failed. Returned {response.status_code}")
                return
        else:
            log("Direct access successful")
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
