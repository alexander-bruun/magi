#!/usr/bin/env python3
"""
ResetScans scraper for MAGI.

Downloads manga/manhwa/manhua from reset-scans.org.
"""

# Standard library imports
import asyncio
import re
import json

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
CONFIG = get_scraper_config("resetscans", "ResetScans", "[ResetScans]")
ALLOWED_DOMAINS = ["reset-scans.org"]
BASE_URL = "https://reset-scans.org"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract all series URLs by fetching all pages.

    Args:
        session: requests.Session object
        page_num: Ignored, always fetch all pages

    Returns:
        list: List of dicts with 'series_url' key
    """
    all_series = []
    page_num = 1
    max_pages = 50  # Safety limit
    
    while page_num <= max_pages:
        if page_num == 1:
            url = "https://reset-scans.org/manga/"
        else:
            url = f"https://reset-scans.org/manga/page/{page_num}/"
        
        log(f"Fetching series list from page {page_num}...")
        
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text
        except Exception as e:
            log(f"Failed to fetch page {page_num}: {e}")
            break
        
        # Extract series URLs - look for manga entry links
        series_urls = re.findall(r'href="https://reset-scans\.org(/manga/[^/]+/)"', html)
        # Filter out chapter URLs and other non-series URLs
        series_urls = [
            url
            for url in series_urls
            if "chapter" not in url and "feed" not in url and "genre" not in url
        ]
        
        series_list = [{'series_url': url} for url in series_urls]
        
        if not series_list:
            log(f"No series found on page {page_num}, stopping")
            break
        
        all_series.extend(series_list)
        log(f"Found {len(series_list)} series on page {page_num}, total: {len(all_series)}")
        
        page_num += 1
    
    log(f"Total series collected: {len(all_series)}")
    # Remove duplicates
    seen = set()
    unique_series = []
    for series in all_series:
        url = series['series_url']
        if url not in seen:
            seen.add(url)
            unique_series.append(series)
    log(f"Unique series: {len(unique_series)}")
    return unique_series, len(unique_series)


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
            .replace(" Manga – Read Online | RESET SCANS", "")
            .strip()
        )
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
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\0", "")

    chapter_urls = re.findall(
        r'href="https://reset-scans\.org(/manga/[^/]+/chapter-[^/]+/)"', html
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
    
    try:
        response = session.get(full_url, timeout=30)
        response.raise_for_status()
    except Exception as e:
        if hasattr(e, 'response') and e.response and e.response.status_code == 403:
            log(f"Cloudflare block detected (403) for chapter page {full_url}, attempting bypass...")
            try:
                cookies, headers = asyncio.run(bypass_cloudflare(full_url))
                if cookies:
                    session.cookies.update(cookies)
                    if headers:
                        session.headers.update(headers)
                    success("Cloudflare bypass re-run successful")
                    # Retry the request
                    response = session.get(full_url, timeout=30)
                    response.raise_for_status()
                else:
                    raise e
            except Exception as bypass_error:
                error(f"Cloudflare bypass re-run failed: {bypass_error}")
                raise e
        else:
            raise e
    
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

    # Look for poster image with "resource" in the src (specific to reset-scans.org posters)
    poster_match = re.search(r'<img[^>]*src="([^"]*resource[^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Reset Scans scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://reset-scans.org", timeout=30)
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
