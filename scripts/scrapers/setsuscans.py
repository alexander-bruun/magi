#!/usr/bin/env python3
"""
SetsuScans scraper for MAGI.

Downloads manga/manhwa/manhua from setsuscans.com via AJAX endpoints.
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
CONFIG = get_scraper_config("setsuscans", "SetsuScans", "[SetsuScans]")
ALLOWED_DOMAINS = ["setsuscans.com"]
BASE_URL = "https://setsuscans.com"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series URLs using AJAX pagination.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    all_series_urls = []
    page = 1
    
    while True:
        url = "https://setsuscans.com/wp-admin/admin-ajax.php"
        data = {
            "action": "madara_load_more",
            "page": page,
            "template": "madara-core/content/content-archive",
            "vars[paged]": page,
            "vars[orderby]": "date",
            "vars[template]": "archive",
            "vars[sidebar]": "right",
            "vars[post_type]": "wp-manga",
            "vars[post_status]": "publish",
            "vars[meta_query][relation]": "AND",
            "vars[manga_archives_item_layout]": "big_thumbnail",
        }

        headers = {
            "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
            "X-Requested-With": "XMLHttpRequest",
            "Referer": "https://setsuscans.com/?m_orderby=new-manga",
        }

        log(f"Fetching series list from page {page}...")

        try:
            response = session.post(url, data=data, headers=headers, timeout=30)
            response.raise_for_status()
            html = response.text

            # Check if this is the last page (no more content)
            is_last_page = len(html.strip()) < 100  # Empty or minimal response

            # Extract series URLs from the HTML response
            series_urls = re.findall(r'href="https://setsuscans\.com(/manga/[^/]+/)"', html)
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
            if page > 50:  # Safety limit
                log("Reached safety limit of 50 pages, stopping.")
                break
                
        except Exception as e:
            error(f"Error fetching page {page}: {e}")
            break
    
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
    try:
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Try multiple selectors for title
        title_match = re.search(
            r'<h1[^>]*class="[^"]*entry-title[^"]*"[^>]*>([^<]+)</h1>', html
        )
        if not title_match:
            title_match = re.search(r"<title>([^<]+)", html)

        if title_match:
            title = (
                title_match.group(1)
                .replace(" – Setsu Scans", "")
                .replace(" | Setsu Scans", "")
                .strip()
            )
            return title
    except Exception as e:
        error(f"Error extracting title from {url}: {e}")

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, manga_url):
    """
    Extract chapter URLs using AJAX endpoint.

    Args:
        session: requests.Session object
        manga_url: Relative URL of the manga

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    # Extract manga slug from URL
    slug_match = re.search(r"/manga/([^/]+)/", manga_url)
    if not slug_match:
        return []

    slug = slug_match.group(1)
    url = f"{BASE_URL}/manga/{slug}/ajax/chapters/"

    headers = {
        "X-Requested-With": "XMLHttpRequest",
        "Referer": f"https://setsuscans.com/manga/{slug}/",
    }

    try:
        response = session.post(url, headers=headers, timeout=30)
        response.raise_for_status()
        html = response.text

        # Extract chapter URLs from the AJAX response
        chapter_urls = re.findall(
            r'href="https://setsuscans\.com(/manga/[^/]+/chapter-[^/]+/)"', html
        )
        
        # Convert to dicts with url and num
        chapter_dicts = []
        for url in chapter_urls:
            chapter_num = extract_chapter_number(url)
            if chapter_num > 0:
                chapter_dicts.append({'url': url, 'num': chapter_num})
        
        # Remove duplicates and sort by chapter number
        unique_dicts = []
        seen_urls = set()
        for chapter in sorted(chapter_dicts, key=lambda x: x['num']):
            if chapter['url'] not in seen_urls:
                unique_dicts.append(chapter)
                seen_urls.add(chapter['url'])
        
        return unique_dicts

    except Exception as e:
        error(f"Error extracting chapters for {manga_url}: {e}")
        return []


def extract_chapter_number(url):
    """
    Extract chapter number from URL for sorting.

    Args:
        url: Chapter URL

    Returns:
        int: Chapter number, or 0 if not found
    """
    match = re.search(r"chapter-(\d+)", url)
    if match:
        return int(match.group(1))
    return 0


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
        html = response.text.replace("\0", "")

        # Look for img src attributes that contain manga images
        image_urls = re.findall(
            r'<img[^>]*src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html
        )

        # Clean URLs and filter for WP-manga images
        cleaned_urls = []
        for url in image_urls:
            url = url.strip()
            if "WP-manga" in url and "thumbnails" not in url and "data/manga" in url:
                # Remove any leading/trailing whitespace or encoded characters
                url = re.sub(r"^[^h]*", "", url)  # Remove anything before 'http'
                if url.startswith("http"):
                    cleaned_urls.append(url)

        return list(dict.fromkeys(cleaned_urls))  # unique

    except Exception as e:
        error(f"Error extracting images from {chapter_url}: {e}")
        return []


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
    try:
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Look for poster image with class="img-responsive"
        poster_match = re.search(r'<img[^>]*class="img-responsive[^"]*"[^>]*src="([^"]+)"', html)
        if poster_match:
            poster_url = poster_match.group(1)
            return poster_url

    except Exception as e:
        error(f"Error extracting poster from {url}: {e}")

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Setsu Scans scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://setsuscans.com", timeout=30)
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
