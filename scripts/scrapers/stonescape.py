#!/usr/bin/env python3
"""
StoneScape scraper for MAGI.

Downloads manga/manhwa/manhua from stonescape.xyz.
"""

# Standard library imports
import asyncio
import re
import time
from urllib.parse import urljoin

# Local imports
from scraper_utils import (
    MAX_RETRIES,
    bypass_cloudflare,
    error,
    get_default_headers,
    get_scraper_config,
    get_session,
    log,
    run_scraper,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("stonescape", "StoneScape", "[StoneScape]")
ALLOWED_DOMAINS = ["stonescape.xyz"]
BASE_URL = "https://stonescape.xyz"


# =============================================================================
# Utility Functions
# =============================================================================
def retry_request(url, session, max_retries=MAX_RETRIES, timeout=60):
    """
    Make a request with retry logic and exponential backoff.

    Args:
        url: URL to request
        session: requests.Session object
        max_retries: Maximum number of retry attempts
        timeout: Request timeout in seconds

    Returns:
        requests.Response object
    """
    for attempt in range(max_retries):
        try:
            response = session.get(url, timeout=timeout)
            if response.status_code == 429:
                wait_time = 2**attempt  # Exponential backoff: 1s, 2s, 4s
                warn(
                    f"Rate limited (429). Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})"
                )
                time.sleep(wait_time)
                continue
            response.raise_for_status()
            return response
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = 2**attempt
                warn(
                    f"Request failed: {e}. Retrying in {wait_time}s... (attempt {attempt + 1}/{max_retries})"
                )
                time.sleep(wait_time)
            else:
                raise e


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
            url = "https://stonescape.xyz/series/"
        else:
            url = f"https://stonescape.xyz/series/page/{page}/"

        try:
            response = retry_request(url, session)
        except Exception as e:
            # If page doesn't exist, we've reached the end
            if "404" in str(e) or "Not Found" in str(e):
                break
            raise

        html = response.text.replace("\n", "")

        # Extract series URLs from the listing
        series_urls = []

        # Look for href links to series
        href_pattern = r"href=\"(https://stonescape\.xyz/series/[^\"]+)\""
        href_links = re.findall(href_pattern, html)

        # Filter to unique series (not chapters)
        series_set = set()
        for link in href_links:
            # Remove the base URL
            relative_link = link.replace("https://stonescape.xyz", "")
            # Check if it's a series link (not a chapter, not feed, not page)
            if (
                "/ch-" not in relative_link
                and not relative_link.endswith("/series/")
                and "/feed" not in relative_link
                and "/page/" not in relative_link
            ):
                series_set.add(relative_link)

        if not series_set:
            break
            
        # Add to all series as dicts
        for url in series_set:
            all_series_urls.append({'series_url': url})
            
        # Check if there's a next page
        has_next_page = (
            f"/series/page/{page + 1}/" in html or f"page/{page + 1}" in html
        )
        
        if not has_next_page:
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
    full_url = urljoin(BASE_URL, series_url)

    response = retry_request(full_url, session)
    html = response.text.replace("\n", "")

    # Try to extract title from various patterns
    title_patterns = [
        r"<h1[^>]*>([^<]*)</h1>",
        r"<title>([^|]*)\|",
        r'"title":"([^"]*)"',
        r'<meta property="og:title" content="([^"]*)"',
    ]

    for pattern in title_patterns:
        match = re.search(pattern, html, re.IGNORECASE)
        if match:
            title = match.group(1).strip()
            # Clean up common suffixes
            title = re.sub(r"\s*\|\s*StoneScape.*$", "", title, re.IGNORECASE)
            return title

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from the series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    full_url = urljoin(BASE_URL, series_url)

    # Get the series slug from the URL
    series_slug = series_url.strip("/").split("/")[-1]

    # Try to get chapters via AJAX endpoint
    ajax_url = f"{full_url}ajax/chapters/"

    chapter_urls = []

    try:
        # POST request to AJAX endpoint
        headers = {
            "accept": "*/*",
            "accept-language": "en-GB,en-US;q=0.9,en;q=0.8",
            "content-length": "0",
            "dnt": "1",
            "origin": BASE_URL,
            "priority": "u=1, i",
            "referer": full_url,
            "sec-ch-ua": '"Chromium";v="143", "Not A(Brand";v="24"',
            "sec-ch-ua-mobile": "?0",
            "sec-ch-ua-platform": '"Windows"',
            "sec-fetch-dest": "empty",
            "sec-fetch-mode": "cors",
            "sec-fetch-site": "same-origin",
            "user-agent": get_default_headers()["User-Agent"],
            "x-requested-with": "XMLHttpRequest",
        }

        response = session.post(ajax_url, headers=headers, timeout=30)

        if response.status_code == 200:
            try:
                # Try to parse as JSON first
                try:
                    data = response.json()
                    if isinstance(data, dict) and "data" in data:
                        html_content = data["data"]
                    else:
                        html_content = response.text
                except:
                    # If not JSON, treat as HTML
                    html_content = response.text

                # Parse chapter links from the HTML content
                chapter_links = re.findall(r'href="([^"]*?/ch-[^"]*?)"', html_content)

                for link in chapter_links:
                    if f"/series/{series_slug}/" in link and "/ch-" in link:
                        # Convert to relative URL
                        if link.startswith("http"):
                            relative_url = link.replace(BASE_URL, "")
                        else:
                            relative_url = link

                        if relative_url not in chapter_urls:
                            chapter_urls.append(relative_url)

                if chapter_urls:  # If we found chapters via AJAX, return them
                    # Convert to dicts with url and num
                    chapter_dicts = []
                    for url in chapter_urls:
                        match = re.search(r"ch-(\d+)", url)
                        if match:
                            chapter_num = int(match.group(1))
                            chapter_dicts.append({'url': url, 'num': chapter_num})
                    
                    # Sort by chapter number
                    chapter_dicts.sort(key=lambda x: x['num'])
                    return chapter_dicts

            except Exception as e:
                pass  # Silently fail and try fallback

    except Exception as e:
        pass  # Silently fail and try fallback

    # Fallback: Try incremental chapter discovery
    chapter_dicts = []
    max_chapters_to_check = 50  # Reasonable limit
    consecutive_empty = 0

    for chapter_num in range(1, max_chapters_to_check + 1):
        chapter_url = f"/series/{series_slug}/ch-{chapter_num}/"
        full_chapter_url = urljoin(BASE_URL, chapter_url)

        try:
            # GET request to check if chapter actually exists and has content
            response = session.get(full_chapter_url, timeout=10)
            if response.status_code == 200:
                html = response.text

                # Check if this is actually a valid chapter page
                # Look for manga ID and actual chapter content
                has_manga_id = "manga_" in html
                has_chapter_indicator = "ch-" in html or "chapter" in html.lower()
                has_images = "<img" in html and ("data-src" in html or "src=" in html)

                # More strict validation: must have manga ID AND images
                if has_manga_id and has_images:
                    chapter_dicts.append({'url': chapter_url, 'num': chapter_num})
                    consecutive_empty = 0
                else:
                    consecutive_empty += 1
            else:
                consecutive_empty += 1

            # Stop if we get 3 consecutive empty/invalid chapters
            if consecutive_empty >= 3:
                break

        except Exception as e:
            consecutive_empty += 1
            if consecutive_empty >= 3:
                break

    # Sort by chapter number
    chapter_dicts.sort(key=lambda x: x['num'])

    return chapter_dicts


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: List of image URLs, None if locked, empty list if not found
    """
    full_url = urljoin(BASE_URL, chapter_url)

    for attempt in range(MAX_RETRIES):
        try:
            response = retry_request(full_url, session)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text

            # Check if chapter is locked or unavailable
            locked_indicators = [
                "this chapter is locked",
                "chapter locked",
                "unlock this chapter",
                "premium content",
                "members only",
            ]
            is_locked = any(
                indicator in html.lower() for indicator in locked_indicators
            )
            if is_locked:
                return None  # Locked/unavailable

            # Extract image URLs - try various patterns
            images = []

            # Look for img tags with src - focus on WP-manga chapter images
            img_matches = re.findall(
                r"<img[^>]*src=\"([^\"]+)\"[^>]*class=\"wp-manga-chapter-img\"",
                html,
                re.IGNORECASE | re.DOTALL,
            )

            for img_url in img_matches:
                img_url = img_url.strip()  # Remove whitespace
                if any(
                    img_url.endswith(ext)
                    for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                ):
                    # Filter out UI elements, logos, watermarks, etc.
                    skip_patterns = [
                        "logo",
                        "banner",
                        "icon",
                        "button",
                        "watermark",
                        "placeholder",
                        "loading",
                        "avatar",
                        "thumb",
                    ]
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        # Only include images that look like chapter pages (numbered files or manga_ in URL)
                        filename = img_url.split("/")[-1].split(".")[
                            0
                        ]  # Get filename without extension
                        if (
                            re.match(r"^\d+$", filename)
                            or "manga_" in img_url
                            or filename in ["0-black"]
                        ):
                            images.append(img_url)

            # Also try the general img src pattern as fallback
            if not images:
                img_matches = re.findall(
                    r"<img[^>]*src=\"([^\"]+)\"", html, re.IGNORECASE | re.DOTALL
                )

                for img_url in img_matches:
                    img_url = img_url.strip()
                    if any(
                        img_url.endswith(ext)
                        for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                    ):
                        skip_patterns = [
                            "logo",
                            "banner",
                            "icon",
                            "button",
                            "watermark",
                            "placeholder",
                            "loading",
                            "avatar",
                            "thumb",
                        ]
                        if not any(skip in img_url.lower() for skip in skip_patterns):
                            filename = img_url.split("/")[-1].split(".")[0]
                            if (
                                re.match(r"^\d+$", filename)
                                or "manga_" in img_url
                                or filename in ["0-black"]
                            ):
                                images.append(img_url)

            # Also look for data-src or similar lazy loading attributes
            data_src_matches = re.findall(
                r"data-src=\"([^\"]+)\"", html, re.IGNORECASE | re.DOTALL
            )

            for img_url in data_src_matches:
                img_url = img_url.strip()
                if any(
                    img_url.endswith(ext)
                    for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                ):
                    skip_patterns = [
                        "logo",
                        "banner",
                        "icon",
                        "button",
                        "watermark",
                        "placeholder",
                        "loading",
                        "avatar",
                        "thumb",
                    ]
                    if not any(skip in img_url.lower() for skip in skip_patterns):
                        filename = img_url.split("/")[-1].split(".")[0]
                        if (
                            re.match(r"^\d+$", filename)
                            or "manga_" in img_url
                            or filename in ["0-black"]
                        ):
                            images.append(img_url)

            # Look for images in other lazy loading attributes
            lazy_attrs = ["data-lazy-src", "data-original", "data-url"]
            for attr in lazy_attrs:
                matches = re.findall(
                    f'{attr}="([^"]+)"', html, re.IGNORECASE | re.DOTALL
                )
                for img_url in matches:
                    img_url = img_url.strip()
                    if any(
                        img_url.endswith(ext)
                        for ext in [".webp", ".jpg", ".png", ".jpeg", ".avif"]
                    ):
                        skip_patterns = [
                            "logo",
                            "banner",
                            "icon",
                            "button",
                            "watermark",
                            "placeholder",
                            "loading",
                            "avatar",
                            "thumb",
                        ]
                        if not any(skip in img_url.lower() for skip in skip_patterns):
                            filename = img_url.split("/")[-1].split(".")[0]
                            if (
                                re.match(r"^\d+$", filename)
                                or "manga_" in img_url
                                or filename in ["0-black"]
                            ):
                                images.append(img_url)

            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(images))

            if len(unique_images) >= 1:
                return unique_images
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(4)

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
    full_url = urljoin(BASE_URL, series_url)

    response = retry_request(full_url, session)
    html = response.text

    # Look for poster image from allowed domains
    for domain in ALLOWED_DOMAINS:
        poster_match = re.search(rf'<img[^>]*src="([^"]*{re.escape(domain)}[^"]*)"', html)
        if poster_match:
            poster_url = poster_match.group(1)
            return poster_url

    return None


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting StoneScape scraper")

    # Health check and Cloudflare bypass
    log(f"Performing health check on {BASE_URL}...")
    try:
        cookies, headers = asyncio.run(bypass_cloudflare(BASE_URL))
        if not cookies:
            return
        session = get_session(cookies, headers)
        response = session.get("https://stonescape.xyz", timeout=30)
        if response.status_code != 200:
            error(f"Health check failed. Returned {response.status_code}")
            return
    except Exception as e:
        error(f"Health check failed: {e}")
        return

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
