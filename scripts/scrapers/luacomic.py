#!/usr/bin/env python3
"""
LuaComic scraper for MAGI.

Downloads manga/manhwa/manhua from luacomic.org.
"""

# Standard library imports
import asyncio
import os
import re
import time
import urllib.parse
from pathlib import Path

# Third-party imports
import requests

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    calculate_padding_width,
    check_duplicate_series,
    download_chapter_images,
    error,
    get_scraper_config,
    get_session,
    log_existing_chapters,
    log_scraper_summary,
    log,
    MAX_RETRIES,
    run_scraper,
    sanitize_title,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("luacomic", "LuaComic", "[LuaComic]")
ALLOWED_DOMAINS = ["media.luacomic.org"]
API_BASE = os.getenv("api", "https://api.luacomic.org")
BASE_URL = "https://luacomic.org"
LUACOMIC_SESSION = os.getenv("LUACOMIC_SESSION")
LUACOMIC_CF_CLEARANCE = os.getenv("LUACOMIC_CF_CLEARANCE")


# =============================================================================
# Authentication Helpers
# =============================================================================
def get_auth_cookies(bypass_cookies=None):
    """
    Get authentication cookies from bypass cookies or environment variables.

    Args:
        bypass_cookies: Optional dict of cookies from Cloudflare bypass

    Returns:
        dict: Authentication cookies
    """
    # First try to get from bypass cookies
    if bypass_cookies:
        ts_session = bypass_cookies.get("ts-session")
        cf_clearance = bypass_cookies.get("cf_clearance")
        if ts_session and cf_clearance:
            # URL decode the ts-session cookie
            ts_session = urllib.parse.unquote(ts_session)
            return {"ts-session": ts_session, "cf_clearance": cf_clearance}

    # Fall back to environment variables
    session_cookie = os.getenv("LUACOMIC_SESSION")
    cf_clearance = os.getenv("LUACOMIC_CF_CLEARANCE")

    if session_cookie and cf_clearance:
        # URL decode the ts-session cookie
        session_cookie = urllib.parse.unquote(session_cookie)
        return {"ts-session": session_cookie, "cf_clearance": cf_clearance}

    # No cookies available
    return {}


def retry_request(
    session, method, url, max_retries=MAX_RETRIES, base_delay=1, **kwargs
):
    """
    Retry a request with exponential backoff for rate limiting.

    Args:
        session: requests.Session object
        method: HTTP method (get, post, etc.)
        url: URL to request
        max_retries: Maximum number of retries
        base_delay: Base delay for exponential backoff
        **kwargs: Additional arguments for the request

    Returns:
        requests.Response object

    Raises:
        requests.exceptions.RequestException: If all retries fail
    """
    for attempt in range(max_retries):
        try:
            response = getattr(session, method.lower())(url, **kwargs)
            response.raise_for_status()
            return response
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 429:  # Too Many Requests
                if attempt < max_retries - 1:
                    delay = base_delay * (2**attempt)  # Exponential backoff
                    warn(
                        f"Rate limited (429). Retrying in {delay}s... (attempt {attempt + 1}/{max_retries})"
                    )
                    time.sleep(delay)
                    continue
                else:
                    error(f"Rate limited after {max_retries} attempts: {url}")
                    raise
            else:
                # Other HTTP errors, don't retry
                raise
        except requests.exceptions.RequestException as e:
            if attempt < max_retries - 1:
                delay = base_delay * (2**attempt)
                warn(
                    f"Request failed: {e}. Retrying in {delay}s... (attempt {attempt + 1}/{max_retries})"
                )
                time.sleep(delay)
                continue
            else:
                error(f"Request failed after {max_retries} attempts: {url}")
                raise


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session):
    """
    Extract all series data from the API with pagination support.

    Args:
        session: requests.Session object

    Returns:
        list: Series info dicts with 'series_url' key
    """
    all_series = []
    current_page = 1

    while True:
        url = f"{API_BASE}/query?page={current_page}&perPage=20&series_type=Comic&query_string=&orderBy=created_at&adult=true&status=All&tags_ids=%5B%5D"

        # Set the required headers from the curl command
        headers = {
            "accept": "application/json, text/plain, */*",
            "accept-language": "en-GB,en-US;q=0.9,en;q=0.8",
            "dnt": "1",
            "origin": "https://luacomic.org",
            "priority": "u=1, i",
            "referer": "https://luacomic.org/",
            "sec-ch-ua": '"Chromium";v="143", "Not A(Brand";v="24"',
            "sec-ch-ua-mobile": "?0",
            "sec-ch-ua-platform": '"Windows"',
            "sec-fetch-dest": "empty",
            "sec-fetch-mode": "cors",
            "sec-fetch-site": "same-site",
            "user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36",
        }

        log(f"Fetching series from API page {current_page}...")
        response = retry_request(session, "get", url, headers=headers, timeout=60)

        data = response.json()
        meta = data.get("meta", {})
        series_list = data.get("data", [])

        if not series_list:
            break

        for series in series_list:
            series_slug = series["series_slug"]
            all_series.append({'series_url': f"/series/{series_slug}"})

        current_page_val = meta.get("current_page", current_page)
        last_page = meta.get("last_page", current_page)
        if current_page_val >= last_page:
            break

        current_page += 1

    return all_series


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
    
    # Fetch series data from API - we need to get the series by slug
    # This is a bit tricky since the API doesn't have a direct slug lookup
    # For now, we'll construct the title from the slug
    # In a real implementation, we'd need to search the API
    return series_slug.replace('-', ' ').title()


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Poster URL, or None if not found
    """
    try:
        # Fetch the series page HTML
        response = session.get(series_url, timeout=60)
        response.raise_for_status()
        html = response.text
        
        # Look for img tag with specific class pattern
        # The class contains "bg-muted/40" and "rounded" with various height classes
        pattern = r'<img[^>]*class="[^"]*bg-muted/40[^"]*rounded[^"]*"[^>]*src="([^"]+)"'
        match = re.search(pattern, html, re.IGNORECASE)
        
        if match:
            poster_url = match.group(1)
            # Parse Next.js image URL to extract actual image URL
            parsed_url = urllib.parse.urlparse(poster_url)
            if 'url' in urllib.parse.parse_qs(parsed_url.query):
                actual_url = urllib.parse.parse_qs(parsed_url.query)['url'][0]
                return urllib.parse.unquote(actual_url)
            else:
                return poster_url
        
        return None
        
    except Exception as e:
        error(f"Error fetching poster for {series_url}: {e}")
        return None


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
        list: List of dicts with 'url' and 'num' keys
    """
    # Extract series_slug from URL
    match = re.search(r'/series/([^/]+)', series_url)
    if not match:
        return []
    
    series_slug = match.group(1)
    
    # Fetch series data from API to get chapters
    url = f"{API_BASE}/series/{series_slug}"
    headers = {
        "accept": "application/json, text/plain, */*",
        "accept-language": "en-GB,en-US;q=0.9,en;q=0.8",
        "dnt": "1",
        "origin": "https://luacomic.org",
        "priority": "u=1, i",
        "referer": "https://luacomic.org/",
        "sec-ch-ua": '"Chromium";v="143", "Not A(Brand";v="24"',
        "sec-ch-ua-mobile": "?0",
        "sec-ch-ua-platform": '"Windows"',
        "sec-fetch-dest": "empty",
        "sec-fetch-mode": "cors",
        "sec-fetch-site": "same-site",
        "user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36",
    }
    
    try:
        response = retry_request(session, "get", url, headers=headers, timeout=60)
        series_data = response.json()
        
        chapters = series_data.get("chapters", [])
        chapter_info = []
        
        for chapter in chapters:
            chapter_slug = chapter.get("chapter_slug", "")
            chapter_name = chapter.get("chapter_name", "")
            
            # Extract chapter number
            match = re.search(r"Chapter (\d+)", chapter_name)
            if match:
                num = int(match.group(1))
            else:
                # Try index
                index = chapter.get("index")
                if index and isinstance(index, (int, float)):
                    num = int(float(index))
                else:
                    continue
            
            chapter_url = f"https://luacomic.org/series/{series_slug}/{chapter_slug}"
            chapter_info.append({'url': chapter_url, 'num': num})
        
        # Sort by chapter number
        chapter_info.sort(key=lambda x: x['num'])
        return chapter_info
        
    except Exception as e:
        error(f"Error fetching chapters for {series_slug}: {e}")
        return []


def extract_image_urls(session, chapter_data, series_data):
    """
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_data: Chapter data dict
        series_data: Series data dict

    Returns:
        list: Image URLs
    """
    series_slug = series_data.get("series_slug")
    chapter_slug = chapter_data.get("chapter_slug")
    chapter_url = f"https://luacomic.org/series/{series_slug}/{chapter_slug}"

    response = retry_request(session, "get", chapter_url, timeout=60)
    html = response.text

    # Check if premium
    if chapter_data.get("is_premium", False):
        return []

    # Extract image URLs from src attributes
    image_urls = re.findall(r'src="https://media\.luacomic\.org/file/[^"]+', html)

    # Remove src=" prefix
    image_urls = [url.replace('src="', "") for url in image_urls]

    # Remove thumbnail if present
    thumbnail = series_data.get("thumbnail", "")
    if thumbnail:
        image_urls = [url for url in image_urls if url != thumbnail]

    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting LuaComic scraper")

    # Cloudflare bypass and authentication setup for API access
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies or "cf_clearance" not in cookies:
            warn("Cloudflare bypass failed, using fallback cookies...")
            # Use fallback cookies from environment variables
            cookies = get_auth_cookies()
            session = get_session(cookies)
        else:
            # Use bypass cookies directly (they should contain both cf_clearance and ts-session)
            # If bypass cookies don't have ts-session, try to get it from auth cookies
            auth_cookies = get_auth_cookies(cookies)
            if auth_cookies:
                cookies.update(auth_cookies)
            session = get_session(cookies, headers)
            log("Cloudflare bypass successful")
            log(f"Obtained cookies: {list(cookies.keys())}")
    except Exception as e:
        warn(f"Cloudflare bypass failed: {e}, using fallback cookies...")
        cookies = get_auth_cookies()
        session = get_session(cookies)

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
