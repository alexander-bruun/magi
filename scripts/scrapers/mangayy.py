#!/usr/bin/env python3
"""
MangaYY scraper for MAGI.

Downloads manga/manhwa/manhua from mangayy.org.
"""

# Standard library imports
import asyncio
import html as html_module
import re
from urllib.parse import urlparse

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
CONFIG = get_scraper_config("mangayy", "MangaYY", "[MangaYY]")
ALLOWED_DOMAINS = ["mangayy.org", "yy.mangayy.org"]
BASE_URL = "https://mangayy.org"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from the manga listing page.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        list: Series info dicts with 'series_url' key
    """
    if page_num == 1:
        all_series = []
        current_page = 1

        while True:
            url = f"{BASE_URL}/page/{current_page}/?m_orderby=new-manga"
            log(f"Fetching series list from page {current_page}...")

            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Check if this is the last page by looking for "next" link
            is_last_page = "next page-numbers" not in html and "Next" not in html

            # Extract series URLs - look for manga entry links
            series_urls = re.findall(r'href="https://mangayy\.org(/manga/[^/]+/)"', html)
            # Filter out chapter URLs and other non-series URLs
            page_series = [
                url
                for url in series_urls
                if "chapter" not in url and "feed" not in url and "genre" not in url
            ]

            # Convert to dicts
            for url in page_series:
                all_series.append({'series_url': url})

            if is_last_page:
                break
            current_page += 1

        return list(set(all_series))  # Remove duplicates
    else:
        return []


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

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
        title = title.replace(" – MangaYY", "").strip()
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

    # Look for poster image with class="img-responsive"
    poster_match = re.search(r'<img[^>]*class="img-responsive"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

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
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
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
    try:
        ajax_response = session.post(ajax_url, timeout=30)
        ajax_response.raise_for_status()
        ajax_html = ajax_response.text

        # Extract chapter URLs from AJAX response
        chapter_urls = re.findall(
            r'href="https://mangayy\.org(/manga/'
            + re.escape(manga_slug)
            + r'/chapter-[^/]+/)"',
            ajax_html,
        )

        if chapter_urls:
            chapter_info = []
            for url in set(chapter_urls):
                match = re.search(r"chapter-(\d+)", url)
                if match:
                    num = int(match.group(1))
                    chapter_info.append({'url': url, 'num': num})

            # Sort by chapter number
            chapter_info.sort(key=lambda x: x['num'])
            return chapter_info
    except Exception as e:
        warn(f"AJAX chapter loading failed: {e}, falling back to HTML parsing")

    # Fallback: extract from the original HTML (for sites that don't use AJAX)
    chapter_urls = re.findall(
        r'href="https://mangayy\.org('
        + re.escape(manga_url.rstrip("/"))
        + r'/chapter-[^/]+/)"',
        html,
    )
    chapter_info = []
    for url in set(chapter_urls):
        match = re.search(r"chapter-(\d+)", url)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': url, 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs for a given chapter.

    Args:
        session: requests.Session object
        chapter_url: Relative URL of the chapter

    Returns:
        list: Image URLs
    """
    full_url = f"{BASE_URL}{chapter_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\0", "")

    # Look for img src attributes that contain manga images
    image_urls = re.findall(r'<img[^>]*src="([^"]*\.(?:jpg|jpeg|png|webp)[^"]*)"', html)
    # Clean URLs and filter for manga images from yy.mangayy.org
    cleaned_urls = []
    for url in image_urls:
        url = url.strip()
        parsed = urlparse(url)
        if (
            parsed.scheme in ("http", "https")
            and (parsed.netloc == "yy.mangayy.org" or "WP-manga" in url)
            and "thumbnails" not in parsed.path
        ):
            cleaned_urls.append(url)
    return list(dict.fromkeys(cleaned_urls))  # unique


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting MangaYY scraper")

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
