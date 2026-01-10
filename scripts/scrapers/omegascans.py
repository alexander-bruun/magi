"""
Omega Scans scraper for MAGI.

Downloads manga/manhwa/manhua from omegascans.org.
"""

# Standard library imports
import asyncio
import os
import re
from urllib.parse import unquote, urlparse, parse_qs

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    get_scraper_config,
    error,
    get_session,
    log,
    run_scraper,
    success,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("omegascans", "OmegaScans", "[OmegaScans]")
ALLOWED_DOMAINS = ["api.omegascans.org", "media.omegascans.org"]
BASE_URL = "https://omegascans.org"
API_BASE = os.getenv("api", "https://api.omegascans.org")


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series data from API.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, bool is_last_page)
    """
    # Fetch all series in one go
    if page_num > 1:
        return [], True

    log("Fetching series list from API...")
    url = f"{API_BASE}/query/?page=1&perPage=99999999999"  # Reduced for testing
    try:
        response = session.get(url, timeout=30)
        response.raise_for_status()
        data = response.json()
        log(f"API response received, found {len(data.get('data', []))} series")
    except Exception as e:
        error(f"Failed to fetch series: {e}")
        return [], True

    series_data = []
    for series in data.get("data", []):
        series_type = series.get("series_type", "")
        if series_type != "Novel":  # Skip novels
            series_data.append(series)

    return series_data, True  # is_last_page = True


def extract_series_title(session, series_url):
    """
    Extract series title from series URL.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Series title
    """
    # Extract series_slug from URL
    match = re.search(r'/series/([^/]+)', series_url)
    if not match:
        return ""
    
    series_slug = match.group(1)
    
    # Fetch series data from API
    url = f"{API_BASE}/series/{series_slug}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    series_data = response.json()
    
    return series_data.get("title", "")


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Poster URL or None
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Find the poster img tag - look for img with specific classes
    # The poster img has classes like "w-full bg-muted/40 h-[500px] ..."
    poster_match = re.search(
        r'<img[^>]*class="[^"]*w-full[^"]*"[^>]*src="([^"]+)"[^>]*>',
        html
    )
    if not poster_match:
        return None

    src = poster_match.group(1)
    
    # Parse the src URL - it's like /_next/image?url=ENCODED_URL&w=...&q=...
    parsed = urlparse(src)
    query_params = parse_qs(parsed.query)
    
    if 'url' not in query_params:
        return None
    
    # Get the encoded URL and decode it
    encoded_url = query_params['url'][0]
    poster_url = unquote(encoded_url)
    
    return poster_url


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series URL.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        list: Chapter URLs
    """
    # Extract series_slug from URL
    match = re.search(r'/series/([^/]+)', series_url)
    if not match:
        return []
    
    series_slug = match.group(1)
    
    # Fetch series data to get series_id
    series_api_url = f"{API_BASE}/series/{series_slug}"
    response = session.get(series_api_url, timeout=30)
    response.raise_for_status()
    series_data = response.json()
    
    series_id = series_data.get("id")
    if not series_id:
        return []
    
    # Fetch chapters
    chapters_url = f"{API_BASE}/chapter/query?page=1&perPage=99999999&series_id={series_id}"
    response = session.get(chapters_url, timeout=30)
    response.raise_for_status()
    data = response.json()

    chapter_info = []
    for chapter in data.get("data", []):
        chapter_name_raw = chapter.get("chapter_name", "").strip()
        
        # Skip seasons and decimal chapters
        if "season" in chapter_name_raw.lower() or "." in chapter_name_raw:
            continue
        
        # Extract chapter number from chapter_name_raw
        # Handle formats like "123" or "Chapter 123"
        match = re.search(r'(?:Chapter\s*)?(\d+)', chapter_name_raw)
        if not match:
            continue
        chapter_num = int(match.group(1))
        
        chapter_slug = chapter.get("chapter_slug")
        if chapter_slug:
            chapter_url = f"{BASE_URL}/series/{series_slug}/{chapter_slug}"
            chapter_info.append({'url': chapter_url, 'num': chapter_num})

    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: URL of the chapter

    Returns:
        list: Image URLs in reading order, empty list if premium/unavailable
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Check if premium
    if "This chapter is premium!" in html:
        return []

    # Extract image URLs from src attributes
    api_urls = re.findall(
        r'src="https://api\.omegascans\.org/uploads/series/[^"]+', html
    )
    media_urls = re.findall(r'src="https://media\.omegascans\.org/file/[^"]+', html)

    all_urls = api_urls + media_urls
    # Remove src=" prefix
    all_urls = [url.replace('src="', "") for url in all_urls]

    return all_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Omega Scans scraper")
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
        series_url_builder=lambda data: f"{BASE_URL}/series/{data.get('series_slug')}"
    )


if __name__ == "__main__":
    main()
