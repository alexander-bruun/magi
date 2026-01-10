#!/usr/bin/env python3
import json

"""
Tritinia scraper for MAGI.

Downloads manga/manhwa/manhua from tritinia.org.
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
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("tritinia", "Tritinia", "[Tritinia]")
ALLOWED_DOMAINS = ["tritinia.org"]
BASE_URL = "https://tritinia.org"


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
            # First page: direct fetch
            url = "https://tritinia.org/manga/"
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text
        else:
            # Subsequent pages: load more via AJAX
            ajax_url = "https://tritinia.org/wp-admin/admin-ajax.php"
            data = {
                "action": "madara_load_more",
                "page": page,
                "template": "madara-core/content/content-archive",
                "vars[paged]": page,
                "vars[orderby]": "meta_value_num",
                "vars[template]": "archive",
                "vars[sidebar]": "right",
                "vars[post_type]": "wp-manga",
                "vars[post_status]": "publish",
                "vars[meta_key]": "_latest_update",
                "vars[order]": "desc",
                "vars[meta_query][relation]": "AND",
                "vars[manga_archives_item_layout]": "default",
            }
            response = session.post(ajax_url, data=data, timeout=30)
            response.raise_for_status()
            html = response.text

        # Extract series URLs
        series_urls = re.findall(r'href="https://tritinia\.org/manga/[^"]*/"', html)
        # Remove href=" and " and filter out chapter and feed URLs
        series_urls = [
            url.replace('href="', "").rstrip('"')
            for url in series_urls
            if "/chapter-" not in url and "/ch-" not in url and "/feed/" not in url
        ]

        # Convert to dict format
        for series_url in series_urls:
            all_series_urls.append({'series_url': series_url})

        is_last_page = len(series_urls) == 0
        if is_last_page:
            break
        page += 1

    return sorted(set(all_series_urls), key=lambda x: x['series_url'])


def extract_series_title(session, series_url):
    """
    Extract series title from the series page.

    Args:
        session: requests.Session object
        series_url: URL of the series page

    Returns:
        str: Series title, or None if not found
    """
    from scraper_utils import MAX_RETRIES, RETRY_DELAY
    import time

    for i in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(series_url, timeout=30)
            response.raise_for_status()
            html = response.text

            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = (
                    title_match.group(1).replace(" &#8211; Tritinia Scans", "").strip()
                )
                if title:
                    return title
        except Exception as e:
            if i < MAX_RETRIES:
                warn(
                    f"Failed to extract title (attempt {i}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)

    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Poster URL, or None if not found
    """
    response = session.get(series_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for poster image with class="img-responsive"
    poster_match = re.search(r'<img[^>]*class="[^"]*img-responsive[^"]*"[^>]*src="([^"]+)"', html, re.IGNORECASE)
    if poster_match:
        return poster_match.group(1)

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs for a given series.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    ajax_url = f"{series_url}ajax/chapters/"
    headers = {
        "accept": "*/*",
        "accept-language": "en-GB,en-US;q=0.9,en;q=0.8",
        "x-requested-with": "XMLHttpRequest",
        "origin": "https://tritinia.org",
        "referer": series_url,
        "sec-fetch-dest": "empty",
        "sec-fetch-mode": "cors",
        "sec-fetch-site": "same-origin",
    }
    response = session.post(ajax_url, headers=headers, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter URLs
    chapter_urls = re.findall(
        r'href="https://tritinia\.org/manga/[^"]*ch-[^"]*/"', html
    )
    chapter_urls = [url.replace('href="', "").rstrip('"') for url in chapter_urls]

    # Convert to dict format with chapter numbers
    chapter_info = []
    for url in chapter_urls:
        match = re.search(r"ch-(\d+)", url)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': url, 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """
    Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: URL of the chapter page

    Returns:
        list: List of image URLs
    """
    import json

    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", " ")

    # Extract image URLs from chapter_preloaded_images script
    script_match = re.search(r"chapter_preloaded_images = (\[[\s\S]*?\])", html)
    if script_match:
        try:
            image_urls = json.loads(script_match.group(1))
            # Remove duplicates while preserving order
            return list(dict.fromkeys(image_urls))
        except json.JSONDecodeError:
            pass

    # Fallback: Extract image URLs from data-src attributes
    image_urls = re.findall(
        r'data-src="\s*(https://tritinia\.org/wp-content/uploads/WP-manga/data/[^"]*\.(?:jpg|jpeg|png|webp))',
        html,
    )
    # Remove duplicates while preserving order
    image_urls = list(dict.fromkeys(image_urls))

    # If no data-src images found, try src attributes
    if not image_urls:
        image_urls = re.findall(
            r'src="\s*(https://tritinia\.org/wp-content/uploads/WP-manga/data/[^"]*\.(?:jpg|jpeg|png|webp))',
            html,
        )
        image_urls = list(dict.fromkeys(image_urls))

    return image_urls


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting Tritinia scraper")

    # Health check and Cloudflare bypass
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
        series_url_builder=lambda data: data['series_url']  # data has 'series_url' key
    )


if __name__ == "__main__":
    main()
