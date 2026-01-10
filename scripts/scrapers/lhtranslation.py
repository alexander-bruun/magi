#!/usr/bin/env python3
"""
LHTranslation scraper for MAGI.

Downloads manga/manhwa/manhua from lhtranslation.net.
"""

# Standard library imports
import asyncio
import re
import time

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    get_scraper_config,
    get_session,
    log,
    MAX_RETRIES,
    RETRY_DELAY,
    run_scraper,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("lhtranslation", "LHTranslation", "[LHTranslation]")
ALLOWED_DOMAINS = ["lhtranslation.net"]
BASE_URL = "https://lhtranslation.net"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from listing page with load more.

    Args:
        session: requests.Session object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series info dicts with 'series_url' key, total_pages)
    """
    if page_num == 1:
        all_series_urls = set()
        current_page = 1

        while True:
            if current_page == 1:
                # First page: direct fetch
                url = f"{BASE_URL}/manga/"
                response = session.get(url, timeout=30)
                response.raise_for_status()
                html = response.text
            else:
                # Subsequent pages: load more via AJAX
                ajax_url = f"{BASE_URL}/wp-admin/admin-ajax.php"
                data = {
                    "action": "madara_load_more",
                    "page": current_page,
                    "template": "madara-core/content/content-archive",
                    "vars[paged]": current_page,
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
            series_urls = re.findall(r'href="https://lhtranslation\.net/manga/[^"]*/"', html)
            # Remove href=" and " and filter out chapter and feed URLs
            page_series = [
                url.replace('href="', "").rstrip('"')
                for url in series_urls
                if "/chapter-" not in url and "/feed/" not in url
            ]

            if not page_series:
                break

            # Convert to relative URLs and add to all_series_urls
            for url in page_series:
                relative_url = url.replace(BASE_URL, "")
                all_series_urls.add(relative_url)

            current_page += 1

        # Convert to list of dicts
        series_list = [{'series_url': url} for url in all_series_urls]
        return series_list, 1
    else:
        return [], 1


def extract_series_title(session, series_url):
    """
    Extract series title from series page.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        str: Series title, or None if not found
    """
    for attempt in range(MAX_RETRIES):
        try:
            response = session.get(f"{BASE_URL}{series_url}", timeout=30)
            response.raise_for_status()
            html = response.text

            title_match = re.search(r"<title>([^<]+)", html)
            if title_match:
                title = (
                    title_match.group(1).replace(" &#8211; LHTranslation", "").strip()
                )
                if title:
                    return title
        except Exception as e:
            if attempt < MAX_RETRIES - 1:
                warn(
                    f"Failed to extract title (attempt {attempt + 1}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)

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
    poster_match = re.search(r'<img[^>]*class="img-responsive[^"]*"[^>]*src="([^"]+)"', html)
    if poster_match:
        poster_url = poster_match.group(1)
        return poster_url

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================
def extract_chapter_urls(session, series_url):
    """
    Extract chapter URLs from series page via AJAX.

    Args:
        session: requests.Session object
        series_url: Relative URL of the series

    Returns:
        list: Chapter info dicts with 'url' and 'num' keys, sorted by chapter number
    """
    ajax_url = f"{BASE_URL}{series_url}ajax/chapters/"
    response = session.post(ajax_url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Extract chapter URLs
    chapter_urls = re.findall(
        r'href="https://lhtranslation\.net/manga/[^"]*chapter-[^"]*/"', html
    )
    chapter_urls = [url.replace('href="', "").rstrip('"') for url in chapter_urls]

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
    Extract image URLs from chapter page.

    Args:
        session: requests.Session object
        chapter_url: Full URL of the chapter

    Returns:
        list: Image URLs
    """
    response = session.get(chapter_url, timeout=30)
    response.raise_for_status()
    html = response.text.replace("\n", " ")

    # Extract image URLs from data-src attributes
    image_urls = re.findall(
        r'data-src="\s*(https://lhtranslation\.net/wp-content/uploads/WP-manga/data/[^"]*\.(?:jpg|jpeg|png|webp))',
        html,
    )
    # Remove duplicates while preserving order
    return list(dict.fromkeys(image_urls))


# =============================================================================
# Main Entry Point
# =============================================================================
def main():
    """Main entry point for the scraper."""
    log("Starting LHTranslation scraper")

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
