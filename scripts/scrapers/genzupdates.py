#!/usr/bin/env python3
"""
GenzUpdates scraper for MAGI.

Downloads manga/manhwa/manhua from genzupdates.com.
"""

# Standard library imports
import asyncio
import random
import re
import subprocess
import time
import urllib.parse

import requests
from fake_useragent import UserAgent

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


def curl_get(url, cookies=None, headers=None):
    """
    Fetch URL using Windows curl with cookies and headers, with human-like delays and retries.
    """
    max_retries = 3
    for attempt in range(max_retries):
        # Human-like delay: 2-5 seconds between requests, longer on retries
        base_delay = random.uniform(2, 5)
        retry_delay = attempt * 10  # Additional delay for retries
        total_delay = base_delay + retry_delay
        time.sleep(total_delay)
        
        # Randomize User-Agent for each request
        user_agents = [
            'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36',
            'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0',
            'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/120.0',
            'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        ]
        random_ua = random.choice(user_agents)
        
        cmd = ['curl', '--fail', '-s', '-L', '--http1.0',
               '--user-agent', random_ua,
               '--max-time', '30',  # Timeout after 30 seconds
               url]
        
        if cookies:
            cookie_str = '; '.join(f'{k}={v}' for k, v in cookies.items())
            if cookie_str:
                cmd.extend(['-b', cookie_str])
        
        # Add some random headers to look more human
        accept_languages = ['en-US,en;q=0.9', 'en-US,en;q=0.9,es;q=0.8', 'en-GB,en;q=0.9', 'en-US,en;q=0.9,de;q=0.8']
        random_accept_lang = random.choice(accept_languages)
        
        cmd.extend(['-H', 'Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7'])
        cmd.extend(['-H', f'Accept-Language: {random_accept_lang}'])
        cmd.extend(['-H', 'Accept-Encoding: identity'])
        cmd.extend(['-H', 'DNT: 1'])
        cmd.extend(['-H', 'Connection: keep-alive'])
        cmd.extend(['-H', 'Upgrade-Insecure-Requests: 1'])
        cmd.extend(['-H', 'Sec-Fetch-Dest: document'])
        cmd.extend(['-H', 'Sec-Fetch-Mode: navigate'])
        cmd.extend(['-H', 'Sec-Fetch-Site: none'])
        cmd.extend(['-H', 'Sec-Fetch-User: ?1'])
        cmd.extend(['-H', 'Cache-Control: max-age=0'])
        
        if headers:
            for k, v in headers.items():
                if k.lower() not in ['user-agent', 'accept', 'accept-language', 'accept-encoding', 'dnt', 'connection', 'upgrade-insecure-requests', 'sec-fetch-dest', 'sec-fetch-mode', 'sec-fetch-site', 'sec-fetch-user', 'cache-control']:
                    cmd.extend(['-H', f'{k}: {v}'])
        
        result = subprocess.run(cmd, capture_output=True, text=False)
        html = result.stdout.decode('utf-8', errors='ignore')
        
        if result.returncode == 0:
            return html
        elif result.returncode == 22:  # HTTP error
            if attempt == max_retries - 1:
                # For simplicity, raise HTTPError like requests
                class MockResponse:
                    status_code = 404  # Assume 404
                    text = html
                raise requests.exceptions.HTTPError(response=MockResponse)
        else:
            if attempt == max_retries - 1:
                raise Exception(f"curl failed with code {result.returncode}")
    
    # Should not reach here
    raise Exception("Max retries exceeded")


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

    url = f"{BASE_URL}"
    log("Fetching series list...")
    html = session.get(url).text

    # Extract series URLs
    series_urls = re.findall(r'href="(/series/[^/]+/)"', html)
    all_series_urls = sorted(set(series_urls))
    log(f"Found {len(all_series_urls)} series")

    if len(all_series_urls) == 0:
        with open('/tmp/genzupdates_debug.html', 'w') as f:
            f.write(html)
        log("Debug: HTML written to /tmp/genzupdates_debug.html")

    # Convert to dicts with series_url
    series_data = [{'series_url': url} for url in all_series_urls]
    return series_data, 1


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
    html = session.get(url).text

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
    html = session.get(url).text

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
    html = session.get(url).text

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
    html = session.get(url).text

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

    # Use manual session with headers
    session = get_session()
    session.headers.update({
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7',
        'Accept-Language': 'en-US,en;q=0.9',
        'Accept-Encoding': 'identity',
        'DNT': '1',
        'Connection': 'keep-alive',
        'Upgrade-Insecure-Requests': '1',
        'Sec-Fetch-Dest': 'document',
        'Sec-Fetch-Mode': 'navigate',
        'Sec-Fetch-Site': 'none',
        'Sec-Fetch-User': '?1',
        'Cache-Control': 'max-age=0',
    })

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
