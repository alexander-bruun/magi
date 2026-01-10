#!/usr/bin/env python3
"""
SpiderScans scraper for MAGI.

Downloads manga/manhwa/manhua from spiderscans.xyz.
"""

# Standard library imports
import asyncio
import html as html_module
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
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("spiderscans", "SpiderScans", "[SpiderScans]")
ALLOWED_DOMAINS = ["spiderscans.xyz"]
BASE_URL = "https://spiderscans.xyz"


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
            url = "https://spiderscans.xyz/new/"
        else:
            url = f"https://spiderscans.xyz/new/page/{page}/"
        log(f"Fetching series list from page {page}...")

        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if this is the last page by looking for "next" link
        is_last_page = "next page-numbers" not in html and "Next" not in html

        # Extract series URLs - look for manga entry links
        series_urls = re.findall(r'href="https://spiderscans\.xyz(/manga/[^/]+/)"', html)
        # Filter out chapter URLs and other non-series URLs
        series_urls = [
            url
            for url in series_urls
            if "chapter" not in url and "feed" not in url and "genre" not in url
        ]
        
        if not series_urls:
            break
            
        # Add to all series as dicts
        for url in series_urls:
            all_series_urls.append({'series_url': url})
            
        if is_last_page:
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
        title = html_module.unescape(title_match.group(1))
        title = title.replace(" – spiderscans.xyz", "").strip()
        return title
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

    # First get the manga page to extract the post ID or other needed data
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract manga slug from URL
    manga_slug = manga_url.strip("/").split("/")[-1]

    # Try to get chapters via AJAX
    ajax_url = f"{BASE_URL}{manga_url}ajax/chapters/?t=1"
    chapter_urls = []
    try:
        ajax_response = session.post(ajax_url, timeout=30)
        ajax_response.raise_for_status()
        ajax_html = ajax_response.text

        # Extract chapter URLs from AJAX response
        chapter_urls = re.findall(
            r'href="https://spiderscans\.xyz(/manga/'
            + re.escape(manga_slug)
            + r'/chapter-[^/]+/)"',
            ajax_html,
        )

    except Exception as e:
        warn(f"AJAX chapter loading failed: {e}, falling back to HTML parsing")

    if not chapter_urls:
        # Fallback: extract from the original HTML (for sites that don't use AJAX)
        chapter_urls = re.findall(
            r'href="https://spiderscans\.xyz('
            + re.escape(manga_url.rstrip("/"))
            + r'/chapter-[^/]+/)"',
            html,
        )
    
    # Convert to dicts with url and num
    chapter_dicts = []
    for url in chapter_urls:
        chapter_num_match = re.search(r"chapter-(\d+)", url)
        if chapter_num_match:
            chapter_num = int(chapter_num_match.group(1))
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
    # Clean URLs and filter for WP-manga images
    cleaned_urls = []
    for url in image_urls:
        url = url.strip()
        if "WP-manga" in url and "thumbnails" not in url:
            # Remove any leading/trailing whitespace or encoded characters
            url = re.sub(r"^[^h]*", "", url)  # Remove anything before 'http'
            if url.startswith("http"):
                cleaned_urls.append(url)
    return list(dict.fromkeys(cleaned_urls))  # unique


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

    # Look for poster image with class="img-responsive"
    poster_match = re.search(r'<img[^>]*class="img-responsive[^"]*"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Spider Scans scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://spiderscans.xyz", timeout=30)
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
