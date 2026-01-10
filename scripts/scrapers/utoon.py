#!/usr/bin/env python3
"""
UToon scraper for MAGI.

Downloads manga/manhwa/manhua from utoon.net.
Supports both async browser-based downloads (with Camoufox) and fallback requests-based downloads.
"""

# Standard library imports
import asyncio
import os
import re
import shutil
import time
from pathlib import Path
from urllib.parse import urljoin, urlparse

from camoufox import AsyncCamoufox

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    check_duplicate_series,
    convert_to_webp,
    create_cbz,
    error,
    get_default_headers,
    get_existing_chapters,
    get_scraper_config,
    get_session,
    log_existing_chapters,
    log_scraper_summary,
    log,
    MAX_RETRIES,
    process_chapter,
    process_series,
    run_scraper,
    RETRY_DELAY,
    sanitize_title,
    success,
    warn,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("utoon", "UToon", "[UToon]")
ALLOWED_DOMAINS = ["utoon.net"]
BASE_URL = "https://utoon.net"
USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"

# Performance settings
MAX_CONCURRENT_SERIES = int(os.getenv("max_concurrent_series", "1"))
MAX_CONCURRENT_CHAPTERS = int(os.getenv("max_concurrent_chapters", "2"))
MAX_CONCURRENT_DOWNLOADS = int(os.getenv("max_concurrent_downloads", "3"))


# =============================================================================
# Series Extraction
# =============================================================================


def extract_series_urls(session):
    """Extract all series URLs from all pages.

    Args:
        session: requests.Session object

    Returns:
        list: List of dicts with 'series_url' key
    """
    all_series_urls = []
    page = 1
    while True:
        if page == 1:
            url = f"{BASE_URL}/manga/"
        else:
            url = f"{BASE_URL}/manga/page/{page}/"

        log(f"Fetching series list from page {page}...")

        response = session.get(url, timeout=30)
        if response.status_code >= 400:
            log(
                f"Page {page} returned status {response.status_code}, stopping pagination"
            )
            break

        html = response.text

        # If page has very little content, it's likely not a valid page
        if len(html) < 1000:
            log(
                f"Page {page} has very little content ({len(html)} chars), likely end of pagination"
            )
            break

        # Check if this is the last page by looking for pagination links
        is_last_page = (
            "next page-numbers" not in html and "Next" not in html and "page/" not in html
        )

        # Match series URLs - look for href="/manga/series-slug/"
        series_urls = re.findall(r'href="(/manga/[a-z0-9\-]+/)"', html)
        # Remove duplicates while preserving order
        series_urls = list(dict.fromkeys(series_urls))

        # Filter out non-series pages (like /manga/page/2/, /manga-genre/, etc)
        series_urls = [
            url for url in series_urls if re.match(r"^/manga/[a-z0-9\-]+/$", url)
        ]

        # Convert to dict format
        for series_url in series_urls:
            all_series_urls.append({'series_url': series_url})

        if is_last_page or not series_urls:
            break
        page += 1

    return sorted(set(all_series_urls), key=lambda x: x['series_url'])


async def extract_series_urls_async(page, page_num):
    """Extract series URLs from a listing page using browser.

    Args:
        page: Browser page object
        page_num: Page number to fetch

    Returns:
        tuple: (list of series URL paths, bool is_last_page)
    """
    if page_num == 1:
        url = f"{BASE_URL}/manga/"
    else:
        url = f"{BASE_URL}/manga/page/{page_num}/"

    log(f"Fetching series list from page {page_num}...")

    response = await page.goto(url, wait_until="load")
    if response.status >= 400:
        log(f"Page {page_num} returned status {response.status}, stopping pagination")
        return [], True

    html = await page.content()

    # If page has very little content, it's likely not a valid page
    if len(html) < 1000:
        log(
            f"Page {page_num} has very little content ({len(html)} chars), likely end of pagination"
        )
        return [], True

    # Check if this is the last page by looking for pagination links
    is_last_page = (
        "next page-numbers" not in html and "Next" not in html and "page/" not in html
    )

    # Match series URLs - look for full URLs like https://utoon.net/manga/series-name/
    # Extract as /manga/series-name/ for consistency
    series_urls = re.findall(r'href="https://utoon\.net(/manga/[a-z0-9\-]+/)"', html)

    # Remove duplicates while preserving order
    series_urls = list(dict.fromkeys(series_urls))

    # Filter out non-series pages
    # Exclude: /manga/feed/, /manga/page/N/, /manga-genre/*, /manga-release/*, etc.
    excluded_patterns = [
        r"^/manga/feed/$",
        r"^/manga/page/\d+/$",
        r"^/manga-",  # exclude /manga-genre/, /manga-release/, etc
    ]

    filtered_urls = []
    for url in series_urls:
        is_excluded = False
        for pattern in excluded_patterns:
            if re.match(pattern, url):
                is_excluded = True
                break
        if not is_excluded:
            filtered_urls.append(url)

    return filtered_urls, is_last_page


def extract_series_title(session, series_url):
    """Extract the series title from a series page.

    Args:
        session: requests.Session object
        series_url: URL path to the series page

    Returns:
        str: The series title, or None if extraction failed
    """
    url = f"{BASE_URL}{series_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(url, timeout=30)
            response.raise_for_status()
            html = response.text

            # Extract title from <h1> with class "post-title" or similar
            # Try to extract from title tag first
            title_match = re.search(r"<title>([^<]+)</title>", html)
            if title_match:
                title = title_match.group(1).strip()
                # Remove common suffixes
                title = re.sub(r"\s*-\s*UToon.*$", "", title).strip()
                if title:
                    return title

            # Fallback: try to find heading
            heading_match = re.search(r"<h1[^>]*>([^<]+)</h1>", html)
            if heading_match:
                return heading_match.group(1).strip()

        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract title (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract title after {MAX_RETRIES} attempts: {e}")
                return None

    return None


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL path to the series page

    Returns:
        str: Poster URL, or None if not found
    """
    url = f"{BASE_URL}{series_url}"
    response = session.get(url, timeout=30)
    response.raise_for_status()
    html = response.text

    # Look for img with class="img-responsive"
    img_match = re.search(r'<img[^>]*class="[^"]*img-responsive[^"]*"[^>]*src="([^"]+)"', html, re.IGNORECASE)
    if img_match:
        poster_url = img_match.group(1)
        # Remove size suffix like -193x278 from the filename
        poster_url = re.sub(r'-\d+x\d+', '', poster_url)
        return poster_url

    return None


# =============================================================================
# Chapter Extraction
# =============================================================================


def extract_chapter_urls(session, series_url):
    """Extract chapter URLs from a series page.

    Args:
        session: requests.Session object
        series_url: URL path to the series page

    Returns:
        list: List of dicts with 'url' and 'num' keys
    """
    full_url = f"{BASE_URL}{series_url}"
    response = session.get(full_url, timeout=30)
    response.raise_for_status()
    html = response.text

    series_slug = series_url.strip("/").split("/")[-1]

    # Extract chapter links like /manga/series-slug/chapter-01/
    chapter_patterns = re.findall(rf"{re.escape(series_slug)}/chapter-[\d]+/", html)
    # Convert to full URLs
    chapter_urls = [f"/manga/{pattern}" for pattern in chapter_patterns]

    # Convert to dict format with chapter numbers
    chapter_info = []
    for url in chapter_urls:
        match = re.search(r"chapter-(\d+)", url)
        if match:
            num = int(match.group(1))
            chapter_info.append({'url': url, 'num': num})

    # Sort by chapter number
    chapter_info.sort(key=lambda x: x['num'])
    return chapter_info


def extract_image_urls(session, chapter_url):
    """Extract image URLs from a chapter page.

    Args:
        session: requests.Session object
        chapter_url: URL path to the chapter page

    Returns:
        list: List of image URLs
    """
    full_url = f"{BASE_URL}{chapter_url}"

    for attempt in range(1, MAX_RETRIES + 1):
        try:
            response = session.get(full_url, timeout=30)
            if response.status_code == 404:
                return []  # 404
            response.raise_for_status()
            html = response.text

            # Extract image URLs from img tags with src attribute
            # Pattern: /wp-content/uploads/WP-manga/data/manga_[id]/[hash]/[number].[ext]
            image_urls = re.findall(
                r'src="(https://utoon\.net/wp-content/uploads/WP-manga/data/manga_[a-z0-9]+/[a-z0-9]+/\d+\.(?:jpg|jpeg|png|webp))"',
                html,
            )

            # Remove duplicates while preserving order
            unique_images = list(dict.fromkeys(image_urls))

            if len(unique_images) >= 1:
                return unique_images

        except Exception as e:
            if attempt < MAX_RETRIES:
                warn(
                    f"Failed to extract images (attempt {attempt}/{MAX_RETRIES}), retrying in {RETRY_DELAY}s... Error: {e}"
                )
                time.sleep(RETRY_DELAY)
            else:
                error(f"Failed to extract images after {MAX_RETRIES} attempts: {e}")
                return []

    return []


async def test_cloudflare_bypass():
    """Minimal test to bypass Cloudflare and print the home page."""
    log("Testing Cloudflare bypass for UToon...")

    try:
        from camoufox import AsyncCamoufox
        from camoufox_captcha import solve_captcha
    except ImportError:
        error("camoufox not installed")
        return

    async with AsyncCamoufox(
        headless=True,
        geoip=True,
        humanize=False,
        i_know_what_im_doing=True,
        config={"forceScopeAccess": True},
        disable_coop=True,
    ) as browser:
        page = await browser.new_page()

        log("Navigating to UToon home page...")
        await page.goto(BASE_URL, wait_until="load")

        # Try to solve captcha if present
        captcha_success = await solve_captcha(
            page, captcha_type="cloudflare", challenge_type="interstitial"
        )
        if not captcha_success:
            warn("Captcha solving may have failed, but continuing...")

        # Wait a bit
        await page.wait_for_timeout(2000)

        # Get the page content
        html = await page.content()

        title_match = re.search(r"<title>([^<]+)</title>", html)
        if title_match:
            success("Successfully bypassed Cloudflare!")
            log(f"Home page title: {title_match.group(1)}")
        else:
            log("No title found - may still be blocked")
            if "Attention Required" in html:
                error("Still hitting Cloudflare protection")


async def download_images_concurrent(image_urls, chapter_folder, page):
    """Download images concurrently using aiohttp."""
    import aiohttp

    async def download_single_image(url, filepath, idx, total):
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url) as response:
                    if response.status == 200:
                        data = await response.read()
                        with open(filepath, "wb") as f:
                            f.write(data)
                        return True
                    else:
                        return False
        except Exception as e:
            return False

    tasks = []
    for i, url in enumerate(image_urls):
        idx = i + 1
        url = url.replace(" ", "%20")
        ext = "." + url.split(".")[-1]
        file = chapter_folder / f"{idx:03d}{ext}"
        tasks.append(download_single_image(url, file, idx, len(image_urls)))

    results = await asyncio.gather(*tasks, return_exceptions=True)
    successful = sum(1 for r in results if r is True)
    return successful


async def process_chapter(ch_url, series_directory, clean_title, num, browser_factory):
    """Process a single chapter concurrently."""
    log(f"Starting chapter {num}")
    browser_class = browser_factory()
    async with browser_class as browser:
        try:
            page = await browser.new_page()

            padded = f"{num:02d}"
            name = f"{clean_title} Ch.{padded} {DEFAULT_SUFFIX}"

            # Navigate to chapter
            full_ch_url = f"{BASE_URL}{ch_url}"
            log(f"Navigating to chapter {num}: {full_ch_url}")
            await page.goto(
                full_ch_url, wait_until="load", timeout=60000
            )  # 60 second timeout
            ch_html = await page.content()

            # Debug: check if page loaded correctly
            title_match = re.search(r"<title>([^<]+)</title>", ch_html)
            if title_match:
                log(f"Chapter {num} page title: {title_match.group(1)}")
            else:
                log(f"Chapter {num} has no title tag")

            # Check for Cloudflare challenge
            if "Attention Required" in ch_html or "cf-browser-verification" in ch_html:
                log(f"Chapter {num} hit Cloudflare challenge")

            # Extract image URLs - try broader patterns
            all_img_tags = re.findall(r'<img[^>]*src="([^"]*)"[^>]*>', ch_html)

            # Look for WP-manga images specifically
            wp_manga_images = []
            for img_src in all_img_tags:
                img_src = img_src.strip()  # Remove leading/trailing whitespace
                if "WP-manga" in img_src:
                    # Convert relative URLs to absolute
                    img_src = urljoin("https://utoon.net", img_src)
                    parsed = urlparse(img_src)
                    if parsed.scheme == "https" and parsed.netloc == "utoon.net":
                        wp_manga_images.append(img_src)

            unique_images = list(dict.fromkeys(wp_manga_images))

            await page.close()

            log(f"Chapter {num}: found {len(unique_images)} images")

            if len(unique_images) <= 1:
                log(f"Skipping chapter {num}: {len(unique_images)} images")
                return 0  # Skip chapters with 0 or 1 images

            if DRY_RUN:
                log(f"Chapter {num} [{len(unique_images)} images]")
                return 1

            log(f"Downloading: {chapter_name} [{len(unique_images)} images]")

            # Create chapter directory
            chapter_folder = series_directory / name
            chapter_folder.mkdir(parents=True, exist_ok=True)

            # Download images concurrently
            downloaded = await download_images_concurrent(
                unique_images, chapter_folder, page
            )

            if downloaded != len(unique_images):
                # Clean up on failure
                shutil.rmtree(chapter_folder)
                log(
                    f"Failed to download all images for chapter {num}: {downloaded}/{len(unique_images)}"
                )
                return 0

            # Convert to WebP if enabled
            if CONVERT_TO_WEBP:
                for img_file in chapter_folder.glob("*"):
                    if img_file.suffix.lower() != ".webp":
                        convert_to_webp(img_file)

            # Create CBZ
            if create_cbz(chapter_folder, name, series_directory):
                shutil.rmtree(chapter_folder)

            log(f"Completed chapter {num}")
            return 1

        except Exception as e:
            error(f"Error processing chapter {num}: {e}")
            return 0


async def process_series(
    series_url, browser_factory, series_semaphore, download_semaphore
):
    """Process a single series with concurrent chapter processing."""
    async with series_semaphore:
        browser_class = browser_factory()
        async with browser_class as browser:
            try:
                page = await browser.new_page()

                # Navigate to series page
                full_url = f"{BASE_URL}{series_url}"
                await page.goto(full_url, wait_until="load")
                html = await page.content()

                # Extract title
                title_match = re.search(r"<title>([^<]+)</title>", html)
                if title_match:
                    title = title_match.group(1).strip()
                    title = re.sub(r"\s*-\s*UToon.*$", "", title).strip()
                else:
                    heading_match = re.search(r"<h1[^>]*>([^<]+)</h1>", html)
                    title = heading_match.group(1).strip() if heading_match else None

                if not title:
                    await page.close()
                    return 0

                clean_title = sanitize_title(title)
                log(f"Processing: {clean_title}")

                series_slug = series_url.strip("/").split("/")[-1]
                series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
                series_directory.mkdir(parents=True, exist_ok=True)

                # Extract chapter URLs - try multiple patterns
                chapter_patterns = []

                # Pattern 1: relative URLs like the-baby-raising-a-devil/chapter-21/
                chapter_patterns.extend(
                    re.findall(rf"{re.escape(series_slug)}/chapter-[\d]+/", html)
                )

                # Pattern 2: full URLs like https://utoon.net/manga/the-baby-raising-a-devil/chapter-21/
                full_url_patterns = re.findall(
                    rf"https://utoon\.net/manga/{re.escape(series_slug)}/chapter-[\d]+/",
                    html,
                )
                chapter_patterns.extend(
                    [
                        pattern.replace("https://utoon.net/manga/", "")
                        for pattern in full_url_patterns
                    ]
                )

                # Pattern 3: href attributes
                href_patterns = re.findall(
                    rf'href="[^"]*/manga/{re.escape(series_slug)}/chapter-[\d]+/?"',
                    html,
                )
                for href in href_patterns:
                    # Extract the path part
                    match = re.search(
                        rf"/manga/{re.escape(series_slug)}/chapter-[\d]+/?", href
                    )
                    if match:
                        chapter_patterns.append(match.group(0).lstrip("/manga/"))

                chapter_urls = [f"/manga/{pattern}" for pattern in chapter_patterns]

                def get_chapter_num(url):
                    match = re.search(r"chapter-(\d+)", url)
                    return int(match.group(1)) if match else 0

                chapter_urls.sort(key=get_chapter_num)
                chapter_urls = list(dict.fromkeys(chapter_urls))

                log(f"Found {len(chapter_urls)} chapters in HTML")

                await page.close()

                if not chapter_urls:
                    log("No chapters found, skipping series")
                    return 0

                # Check existing chapters
                existing_chapters = set()
                for cbz_file in series_directory.glob("*.cbz"):
                    match = re.search(r"Ch\.([\d.]+)", cbz_file.stem)
                    if match:
                        existing_chapters.add(float(match.group(1)))

                log(
                    f"Found {len(existing_chapters)} existing chapters: {sorted(existing_chapters) if existing_chapters else 'none'}"
                )

                # Filter out existing chapters
                chapters_to_process = []
                for ch_url in chapter_urls:
                    num_match = re.search(r"chapter-(\d+)", ch_url)
                    if num_match:
                        num = int(num_match.group(1))
                        if num not in existing_chapters:
                            chapters_to_process.append((ch_url, num))

                log(f"Chapters to process: {len(chapters_to_process)}")
                if chapters_to_process:
                    log(f"Sample to process: {chapters_to_process[:3]}")

                if not chapters_to_process:
                    return 0

                # Process chapters concurrently
                chapter_tasks = []
                for ch_url, num in chapters_to_process:
                    task = process_chapter(
                        ch_url, series_directory, clean_title, num, browser_factory
                    )
                    chapter_tasks.append(task)

                # Limit concurrent chapters and add delay between batches
                semaphore = asyncio.Semaphore(MAX_CONCURRENT_CHAPTERS)

                async def limited_task(task):
                    async with semaphore:
                        result = await task
                        await asyncio.sleep(
                            2
                        )  # Delay between chapters to avoid rate limiting
                        return result

                limited_tasks = [limited_task(task) for task in chapter_tasks]
                results = await asyncio.gather(*limited_tasks, return_exceptions=True)

                chapter_count = sum(1 for r in results if isinstance(r, int) and r > 0)

                return chapter_count

            except Exception as e:
                error(f"Error processing series {series_url}: {e}")
                return 0


# =============================================================================
# Main Entry Point
# =============================================================================


def main():
    """Main entry point for the scraper."""
    run_scraper(
        extract_series_urls,
        extract_chapter_urls,
        extract_image_urls,
        extract_series_title,
        extract_poster_url,
        CONFIG,
        ALLOWED_DOMAINS,
        cloudflare_bypass=True,
    )


async def main_async_optimized():
    """Optimized main function with concurrent processing."""
    log("Using Camoufox browser for Cloudflare bypass (optimized version)...")
    log(
        f"Concurrency settings: {MAX_CONCURRENT_SERIES} series, {MAX_CONCURRENT_CHAPTERS} chapters, {MAX_CONCURRENT_DOWNLOADS} downloads"
    )

    try:
        # Create browser factory function
        def create_browser():
            return AsyncCamoufox(
                headless=True,
                geoip=True,
                humanize=False,
                i_know_what_im_doing=True,
                config={"forceScopeAccess": True},
                disable_coop=True,
            )

        # Create semaphores for limiting concurrency
        series_semaphore = asyncio.Semaphore(MAX_CONCURRENT_SERIES)
        download_semaphore = asyncio.Semaphore(MAX_CONCURRENT_DOWNLOADS)

        # Establish Cloudflare session with one browser
        log("Establishing Cloudflare session...")
        temp_browser_class = create_browser()
        async with temp_browser_class as temp_browser:
            temp_page = await temp_browser.new_page()
            await temp_page.goto(BASE_URL, wait_until="load")

            # Try to solve captcha
            from camoufox_captcha import solve_captcha

            captcha_success = await solve_captcha(
                temp_page, captcha_type="cloudflare", challenge_type="interstitial"
            )
            if not captcha_success:
                warn("Captcha solving may have failed, but continuing...")

            await temp_page.wait_for_timeout(2000)
        success("Cloudflare session established")

        # Ensure folder exists
        Path(FOLDER).mkdir(parents=True, exist_ok=True)

        # Collect all series URLs using a single browser session
        log("Collecting all series URLs...")
        all_series_urls = []

        collect_browser_class = create_browser()
        async with collect_browser_class as collect_browser:
            collect_page = await collect_browser.new_page()
            await collect_page.goto(BASE_URL, wait_until="load")

            page_num = 1
            while True:
                try:
                    page_series, is_last_page = await extract_series_urls_async(
                        collect_page, page_num
                    )
                    if page_series:
                        all_series_urls.extend(page_series)
                        log(f"Found {len(page_series)} series on page {page_num}")
                    else:
                        log(f"No series found on page {page_num}")
                        break  # Stop if no series found on this page

                    if is_last_page:
                        break

                    page_num += 1
                except Exception as e:
                    error(f"Error extracting series on page {page_num}: {e}")
                    break

        total_series = len(all_series_urls)
        log(f"Total series found: {total_series}")

        if not all_series_urls:
            return

        # Process all series concurrently with semaphore
        log(f"Processing {len(all_series_urls)} series...")

        series_tasks = [
            process_series(
                series_url, create_browser, series_semaphore, download_semaphore
            )
            for series_url in all_series_urls
        ]
        results = await asyncio.gather(*series_tasks, return_exceptions=True)

        total_chapters = sum(r for r in results if isinstance(r, int))

        log(f"Test series processed - chapters processed: {total_chapters}")
        success(f"Completed! Output: {FOLDER}")

        return  # Exit after testing first series

    except Exception as e:
        error(f"Browser error: {e}")


async def main_async():
    """Main function using Camoufox browser for better Cloudflare handling."""
    log("Using Camoufox browser for Cloudflare bypass...")

    try:
        async with AsyncCamoufox(
            headless=True,
            geoip=True,
            humanize=False,
            i_know_what_im_doing=True,
            config={"forceScopeAccess": True},
            disable_coop=True,
        ) as browser:
            page = await browser.new_page()

            # Navigate to base URL to establish Cloudflare session
            log("Establishing Cloudflare session...")
            await page.goto(BASE_URL, wait_until="load")
            success("Cloudflare session established")

            # Ensure folder exists
            Path(FOLDER).mkdir(parents=True, exist_ok=True)

            # Collect all series URLs
            all_series_urls = []
            page_num = 1
            # Only fetch first page for testing
            try:
                page_series, is_last_page = await extract_series_urls_async(
                    page, page_num
                )
                if page_series:
                    all_series_urls.extend(page_series)
                    log(f"Found {len(page_series)} series on page {page_num}")
                else:
                    log(f"No series found on page {page_num}")
            except Exception as e:
                error(f"Error extracting series on page {page_num}: {e}")

            total_series = len(all_series_urls)
            total_chapters = 0

            # Process each series
            for idx, series_url in enumerate(
                all_series_urls[:1], 1
            ):  # Test with first series only
                log(f"[{idx}/{min(total_series, 1)}] Processing {series_url}")

                # Navigate to series page
                full_url = f"{BASE_URL}{series_url}"
                await page.goto(full_url, wait_until="load")
                html = await page.content()

                # Extract title
                title_match = re.search(r"<title>([^<]+)</title>", html)
                if title_match:
                    title = title_match.group(1).strip()
                    title = re.sub(r"\s*-\s*UToon.*$", "", title).strip()
                else:
                    heading_match = re.search(r"<h1[^>]*>([^<]+)</h1>", html)
                    title = heading_match.group(1).strip() if heading_match else None

                if not title:
                    error("No title → skip")
                    continue

                clean_title = sanitize_title(title)
                log(f"Title: {clean_title}")
                # Check for duplicate in higher priority providers
                if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
                    continue

                series_slug = series_url.strip("/").split("/")[-1]

                # Extract chapter URLs from the HTML
                chapter_patterns = re.findall(
                    rf"{re.escape(series_slug)}/chapter-[\d]+/", html
                )
                chapter_urls = [f"/manga/{pattern}" for pattern in chapter_patterns]

                def get_chapter_num(url):
                    match = re.search(r"chapter-(\d+)", url)
                    return int(match.group(1)) if match else 0

                chapter_urls.sort(key=get_chapter_num)
                chapter_urls = list(dict.fromkeys(chapter_urls))

                if not chapter_urls:
                    log(f"No chapters found → skip")
                    continue

                # Create series directory (only after confirming chapters exist)
                series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
                series_directory.mkdir(parents=True, exist_ok=True)

                # Scan existing CBZ files
                existing_chapters = get_existing_chapters(series_directory)
                log_existing_chapters(existing_chapters)

                chapter_count = 0
                for ch_url in chapter_urls:
                    if chapter_count >= 1:  # Test with first chapter only
                        break
                    num_match = re.search(r"chapter-(\d+)", ch_url)
                    if not num_match:
                        continue
                    num = int(num_match.group(1))

                    if num in existing_chapters:
                        continue

                    padded = f"{num:02d}"
                    name = f"{clean_title} Ch.{padded} {DEFAULT_SUFFIX}"

                    # Navigate to chapter
                    full_ch_url = f"{BASE_URL}{ch_url}"
                    await page.goto(full_ch_url, wait_until="load")
                    ch_html = await page.content()

                    # Extract image URLs - pattern: /WP-manga/data/manga_[id]/[hash]/[number].[ext]
                    image_urls = re.findall(
                        r'src="\s*(https://utoon\.net/wp-content/uploads/WP-manga/data/manga_[a-z0-9]+/[a-z0-9]+/\d+\.(?:jpg|jpeg|png|webp))"',
                        ch_html,
                    )
                    unique_images = list(dict.fromkeys(image_urls))

                    if len(unique_images) == 0:
                        log(f"Skipping: Chapter {num} (not found)")
                        continue
                    elif len(unique_images) == 1:
                        log(f"Skipping: Chapter {num} (only 1 image)")
                        continue

                    chapter_count += 1

                    if DRY_RUN:
                        log(f"Chapter {num} [{len(unique_images)} images]")
                        continue

                    log(f"Downloading: {chapter_name} [{len(unique_images)} images]")

                    # Download images
                    chapter_folder = series_directory / name
                    chapter_folder.mkdir(parents=True, exist_ok=True)

                    downloaded = 0
                    total = len(unique_images)

                    for i, url in enumerate(unique_images):
                        idx_img = i + 1
                        url = url.replace(" ", "%20")
                        ext = "." + url.split(".")[-1]
                        file = chapter_folder / f"{idx_img:03d}{ext}"

                        try:
                            # Use page to get image to maintain session
                            response = await page.evaluate(
                                f"""async () => {{
                                const res = await fetch("{url}");
                                return await res.arrayBuffer();
                            }}"""
                            )
                            with open(file, "wb") as f:
                                f.write(
                                    bytes(response)
                                    if isinstance(response, (list, bytes))
                                    else response
                                )
                            downloaded += 1

                            # Convert to WebP if enabled
                            ext = file.suffix.lower()
                            if CONVERT_TO_WEBP and ext != ".webp":
                                convert_to_webp(file)
                        except Exception as e:
                            # Clean up and break
                            shutil.rmtree(chapter_folder)
                            break

                    if downloaded != total:
                        warn("Incomplete → skipped")
                        continue

                    if create_cbz(chapter_folder, name, series_directory):
                        shutil.rmtree(chapter_folder)
                    else:
                        warn(f"CBZ creation failed for Chapter {num}, keeping folder")

            # Use shared summary logging
            log_scraper_summary(total_series, total_chapters, CONFIG)

    except Exception as e:
        error(f"Browser error: {e}")


def main_requests():
    """Fallback function using requests library."""
    log("Using requests library (Camoufox not available)")

    headers = get_default_headers(USER_AGENT)
    session = get_session(headers=headers)

    # Ensure folder exists
    Path(FOLDER).mkdir(parents=True, exist_ok=True)

    # Collect all series URLs
    all_series_urls = []
    page = 1
    while True:
        try:
            page_series, is_last_page = extract_series_urls(session, page)
            if not page_series:
                log(f"No series found on page {page}, stopping.")
                break
            all_series_urls.extend(page_series)
            log(f"Found {len(page_series)} series on page {page}")

            if is_last_page:
                log(f"Reached last page ({page})")
                break
            page += 1
        except Exception as e:
            error(f"Error extracting series on page {page}: {e}")
            break

    total_series = len(all_series_urls)
    total_chapters = 0

    # Process each series
    for idx, series_url in enumerate(all_series_urls, 1):
        log(f"[{idx}/{total_series}] Processing {series_url}")

        title = extract_series_title(session, series_url)
        if not title:
            error("No title → skip")
            continue

        clean_title = sanitize_title(title)
        log(f"Title: {clean_title}")
        # Check for duplicate in higher priority providers
        if check_duplicate_series(clean_title, HIGHER_PRIORITY_FOLDERS):
            continue

        series_slug = series_url.strip("/").split("/")[-1]

        # Extract chapter URLs
        try:
            chapter_urls = extract_chapter_urls(session, series_url)
        except Exception as e:
            error(f"Error extracting chapters for {series_url}: {e}")
            continue

        if not chapter_urls:
            log(f"No chapters found → skip")
            continue

        # Create series directory (only after confirming chapters exist)
        series_directory = Path(FOLDER) / f"{clean_title} {DEFAULT_SUFFIX}"
        series_directory.mkdir(parents=True, exist_ok=True)

        # Scan existing CBZ files to determine which chapters are already downloaded
        existing_chapters = get_existing_chapters(series_directory)
        log_existing_chapters(existing_chapters)

        for ch_url in chapter_urls:
            # Extract chapter number from URL like /manga/series-slug/chapter-01/
            num_match = re.search(r"chapter-(\d+)", ch_url)
            if not num_match:
                continue
            num = int(num_match.group(1))

            # Skip if chapter already exists
            if num in existing_chapters:
                continue

            padded = f"{num:02d}"
            name = f"{clean_title} Ch.{padded} {DEFAULT_SUFFIX}"

            try:
                imgs = extract_image_urls(session, ch_url)
            except Exception as e:
                error(f"Error extracting images for {ch_url}: {e}")
                continue

            if len(imgs) == 0:
                log(f"Skipping: Chapter {num} (not found)")
                continue
            elif len(imgs) == 1:
                log(f"Skipping: Chapter {num} (only 1 image)")
                continue

            total_chapters += 1

            if DRY_RUN:
                log(f"Chapter {num} [{len(imgs)} images]")
                continue

            log(f"Downloading: {chapter_name} [{len(imgs)} images]")

            # Create chapter directory within series directory
            chapter_folder = series_directory / name
            chapter_folder.mkdir(parents=True, exist_ok=True)

            downloaded = 0
            total = len(imgs)

            for i, url in enumerate(imgs):
                idx = i + 1
                url = url.replace(" ", "%20")
                ext = "." + url.split(".")[-1]
                file = chapter_folder / f"{idx:03d}{ext}"

                try:
                    response = session.get(url, timeout=120)
                    response.raise_for_status()
                    with open(file, "wb") as f:
                        f.write(response.content)
                    downloaded += 1

                    # Convert to WebP if enabled
                    ext = file.suffix.lower()
                    if CONVERT_TO_WEBP and ext != ".webp":
                        convert_to_webp(file)
                except Exception as e:
                    # Clean up and break
                    shutil.rmtree(chapter_folder)
                    break

            if downloaded != total:
                warn("Incomplete → skipped")
                continue

            if create_cbz(chapter_folder, name, series_directory):
                shutil.rmtree(chapter_folder)
            else:
                warn(f"CBZ creation failed for Chapter {num}, keeping folder")

    # Use shared summary logging
    log_scraper_summary(total_series, total_chapters, CONFIG)


if __name__ == "__main__":
    main()
