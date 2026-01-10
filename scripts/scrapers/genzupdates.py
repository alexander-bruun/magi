#!/usr/bin/env python3
"""
GenzUpdates scraper for MAGI.

Downloads manga/manhwa/manhua from genzupdates.com.
"""

# Standard library imports
import asyncio
import re
import urllib.parse

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    get_scraper_config,
    get_session,
    log,
    run_scraper,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("genzupdates", "GenzUpdates", "[GenzUpdates]")
ALLOWED_DOMAINS = ["cdn.meowing.org"]
BASE_URL = "https://genzupdates.com"


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
        if current_page == 1:
            url = f"{BASE_URL}/series"
        else:
            url = f"{BASE_URL}/series/page/{current_page}/"

        log(f"Fetching series list from page {current_page}...")
        response = session.get(url, timeout=30)
        response.raise_for_status()
        html = response.text

        # Check if this is the last page
        is_last_page = (
            "next page-numbers" not in html and "Next" not in html and ">" not in html
        )

        # Extract series URLs
        series_urls = re.findall(r'href="(/series/[^/]+/)"', html)
        all_series_urls.extend(series_urls)

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
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    title_match = re.search(r"<title>([^<]+)", html)
    if title_match:
        title = title_match.group(1).replace(" - Genz Toon", "").strip()
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
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image in style="--photoURL:url(...)"
    poster_match = re.search(r'style="--photoURL:url\(([^)]+)\)"', html)
    if poster_match:
        proxy_url = poster_match.group(1)
        # Parse the query parameter 'url' from the proxy URL
        parsed_url = urllib.parse.urlparse(proxy_url)
        query_params = urllib.parse.parse_qs(parsed_url.query)
        if 'url' in query_params:
            poster_url = query_params['url'][0]
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
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter URLs
    chapter_urls = re.findall(r'href="(/chapter/[^"]+)"', html)

    # Remove duplicates
    unique_urls = list(set(chapter_urls))

    chapter_info = []
    for url in unique_urls:
        # Extract chapter number from URL (last numeric part)
        parts = url.split("-")
        num = 0
        for part in reversed(parts):
            if part.isdigit():
                num = int(part)
                break
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
        list: Image URLs in reading order
    """
    url = f"{BASE_URL}{chapter_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for the #pages div and extract img tags with uid attributes
    pages_div_match = re.search(
        r'<div[^>]*id="pages"[^>]*>(.*?)</div>', html, re.DOTALL | re.IGNORECASE
    )
    if pages_div_match:
        pages_html = pages_div_match.group(1)
        # Extract uid attributes from img tags
        uid_matches = re.findall(
            r'<img[^>]*uid="([^"]+)"[^>]*>', pages_html, re.IGNORECASE
        )

        image_urls = []
        for uid in uid_matches:
            if uid and len(uid.strip()) > 0:
                image_url = f"https://cdn.meowing.org/uploads/{uid}"
                image_urls.append(image_url)

        return image_urls

    # Fallback: look for image ID patterns in the entire HTML
    id_pattern = r"[A-Z][A-Za-z0-9]{9,11}"
    candidates = re.findall(id_pattern, html)

    # Filter candidates to ensure they have mixed case and numbers
    image_ids = []
    for candidate in candidates:
        has_upper = any(c.isupper() for c in candidate)
        has_lower = any(c.islower() for c in candidate)
        has_digit = any(c.isdigit() for c in candidate)
        if has_upper and has_lower and has_digit and len(candidate) >= 10:
            image_ids.append(candidate)

    # Remove duplicates while preserving order
    seen = set()
    unique_ids = []
    for img_id in image_ids:
        if img_id not in seen:
            seen.add(img_id)
            unique_ids.append(img_id)

    # Verify each ID forms a valid image URL
    valid_image_urls = []
    for img_id in unique_ids:
        test_url = f"https://cdn.meowing.org/uploads/{img_id}"
        try:
            img_resp = session.head(test_url, timeout=5)
            if img_resp.status_code == 200:
                valid_image_urls.append(test_url)
        except:
            pass

    return valid_image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting GenzUpdates scraper")

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
