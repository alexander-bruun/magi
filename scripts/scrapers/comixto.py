"""
Comix.to scraper for MAGI.

Downloads manga/manhwa/manhua from comix.to.

The scraper now correctly extracts chapter images from wowpic domains.
Chapter URLs include the chapter_id in the format: /title/{hash}-{slug}/{chapter_id}-chapter-{number}
"""

# Standard library imports
import asyncio
import os
import re
from pathlib import Path
from urllib.parse import unquote, urlparse, parse_qs

# Local imports
from scraper_utils import (
    bypass_cloudflare,
    download_poster,
    get_scraper_config,
    error,
    get_session,
    log,
    process_chapter,
    run_scraper,
    success,
)

# =============================================================================
# Configuration
# =============================================================================
CONFIG = get_scraper_config("comixto", "Comix.to", "[Comix]")
ALLOWED_DOMAINS = ["comix.to", "api.comix.to", "static.comix.to", "wowpic1.store", "wowpic2.store", "wowpic3.store", "wowpic4.store", "wowpic5.store", "wowpic6.store", "wowpic7.store", "wowpic8.store", "wowpic9.store", "wowpic10.store"]
BASE_URL = "https://comix.to"
API_BASE = "https://comix.to/api/v2"


# =============================================================================
# Series Extraction
# =============================================================================
def extract_series_urls(session, page_num):
    """
    Extract series URLs from API.

    Args:
        session: requests.Session object
        page_num: Page number to fetch (1-indexed)

    Returns:
        tuple: (list of series data dicts, bool is_last_page)
    """

    series_data = []
    is_last_page = False
    
    url = f"{API_BASE}/manga?keyword=&order[relevance]=desc&limit=100&page={page_num}"
    try:
        response = session.get(url, timeout=30)
        response.raise_for_status()
        data = response.json()
        if not isinstance(data, dict):
            error(f"Invalid JSON response for series on page {page_num}")
            is_last_page = True
            return series_data, is_last_page
    except Exception as e:
        error(f"Failed to fetch series from page {page_num}: {e}")
        is_last_page = True
        return series_data, is_last_page
    
    result = data.get("result", {})
    if not isinstance(result, dict):
        error(f"Invalid result for series on page {page_num}")
        is_last_page = True
        return series_data, is_last_page
    
    page_series_data = result.get("items", [])
    if not isinstance(page_series_data, list):
        error(f"Invalid items for series on page {page_num}")
        is_last_page = True
        return series_data, is_last_page
    
    for series in page_series_data:
        if not isinstance(series, dict):
            continue
        series_url = f"{BASE_URL}/title/{series.get('hash_id')}-{series.get('slug')}"
        poster_data = series.get("poster", {})
        series_data.append({'series_url': series_url, 'poster_data': poster_data})
        
    log(f"Found {len(page_series_data)} series on page {page_num}")
    
    # Check pagination
    pagination = data.get("result", {}).get("pagination", {})
    current_page = pagination.get("current_page", page_num)
    last_page = pagination.get("last_page", page_num)
    if current_page >= last_page:
        is_last_page = True
        log(f"Reached last page (page {current_page})")
    
    return series_data, is_last_page


def extract_series_title(session, series_url):
    """
    Extract series title from series URL.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Series title
    """
    # For comix.to, the title should be available in the series data passed to run_scraper
    # But as a fallback, extract from URL or API
    # Extract hash-id from URL
    match = re.search(r'/title/([^-]+)-(.+)', series_url)
    if not match:
        return ""
    
    hash_id = match.group(1)
    slug = match.group(2)
    
    # Try to get title from slug (replace hyphens with spaces and capitalize)
    title = slug.replace('-', ' ').title()
    
    return title


def extract_poster_url(session, series_url):
    """
    Extract poster URL from series page.

    Args:
        session: requests.Session object
        series_url: URL of the series

    Returns:
        str: Poster URL or None
    """
    try:
        response = session.get(series_url, timeout=30)
        response.raise_for_status()
        html = response.text
        
        # Extract poster URL from itemprop="image"
        match = re.search(r'<img[^>]*itemprop="image"[^>]*src="([^"]+)"', html)
        if match:
            return match.group(1)
    except Exception as e:
        error(f"Failed to extract poster from {series_url}: {e}")
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
        list: Chapter URLs
    """
    # Extract hash-id from URL
    match = re.search(r'/title/([^-]+)-', series_url)
    if not match:
        return []
    
    hash_id = match.group(1)
    
    # Dict to track the best chapter per number (highest votes)
    best_chapters = {}
    page_num = 1
    max_pages = 100  # Safety limit
    
    while page_num <= max_pages:
        # Fetch chapters
        chapters_url = f"{API_BASE}/manga/{hash_id}/chapters?limit=100&page={page_num}&order[number]=desc"
        try:
            response = session.get(chapters_url, timeout=30)
            response.raise_for_status()
            data = response.json()
            if data is None or not isinstance(data, dict):
                error(f"Invalid JSON response for chapters of {hash_id}")
                break
        except Exception as e:
            error(f"Failed to fetch chapters for {hash_id}: {e}")
            break
        
        result = data.get("result")
        if result is None or not isinstance(result, dict):
            error(f"Invalid result for chapters of {hash_id}")
            break
        
        chapters = result.get("items", [])
        if not isinstance(chapters, list):
            error(f"Invalid items for chapters of {hash_id}")
            break
        if not chapters:
            break
        
        for chapter in chapters:
            if chapter is None or not isinstance(chapter, dict):
                continue
            chapter_num = chapter.get("number")
            chapter_id = chapter.get("chapter_id")
            votes = chapter.get("votes", 0)
            group = (chapter.get("scanlation_group") or {}).get("name", "")
            if chapter_num is not None and chapter_id is not None:
                chapter_num_float = float(chapter_num)
                # Construct chapter URL with chapter_id
                chapter_url = f"{series_url}/{chapter_id}-chapter-{chapter_num}"
                
                # Check if we have a better chapter for this number
                if chapter_num_float not in best_chapters or votes > best_chapters[chapter_num_float]['votes']:
                    best_chapters[chapter_num_float] = {
                        'url': chapter_url,
                        'num': chapter_num_float,
                        'votes': votes,
                        'group': group
                    }
        
        # Check if there are more pages
        pagination = result.get("pagination", {})
        current_page = pagination.get("current_page", page_num)
        last_page = pagination.get("last_page", page_num)
        if current_page >= last_page:
            break
        
        page_num += 1
    
    # Convert to list and sort by chapter number descending
    chapter_info = list(best_chapters.values())
    chapter_info.sort(key=lambda x: x['num'], reverse=True)
    
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
    try:
        response = session.get(chapter_url, timeout=30)
        response.raise_for_status()
        html = response.text
        
        # First priority: Look for wowpic image URLs (real chapter images)
        wowpic_urls = re.findall(r'https?://[^\s\"\'<>]*wowpic[^\s\"\'<>]*\.(?:webp|jpg|jpeg|png)', html, re.IGNORECASE)
        if wowpic_urls:
            log(f"Found {len(wowpic_urls)} wowpic images for chapter")
            return sorted(set(wowpic_urls))  # Remove duplicates and sort
        
        # Fallback: Look for other image URLs in the HTML
        image_urls = re.findall(r'<img[^>]*src="([^"]+\.(?:webp|jpg|jpeg|png))"[^>]*>', html, re.IGNORECASE)
        https_urls = re.findall(r'https://[^"]+\.(?:webp|jpg|jpeg|png)', html, re.IGNORECASE)
        
        all_urls = image_urls + https_urls
        # Remove duplicates and filter
        filtered_urls = []
        seen = set()
        for url in all_urls:
            if url not in seen and ('wowpic' in url or 'comix.to' in url or url.startswith('https://')):
                filtered_urls.append(url)
                seen.add(url)
        
        # Check if these appear to be placeholder/demo images
        # If all images have the same set of hashes, the site is likely showing demo content
        if filtered_urls:
            # Extract image hashes to check for uniqueness
            hashes = set()
            for url in filtered_urls:
                # Extract hash from URLs like /i/d/95/68de795e26ced@280.jpg
                match = re.search(r'/i/[^/]+/[^/]+/([^@]+)', url)
                if match:
                    hash_value = match.group(1)
                    # Remove .jpg extension if present
                    hash_value = hash_value.replace('.jpg', '')
                    hashes.add(hash_value)
            
            # If we have many URLs but very few unique hashes relative to total, likely demo content
            # Also check if we have a known demo hash pattern
            demo_hashes = {'68de558770704', '68de795e26ced', '68dec20d596a5', '68dec2dc2254d', '68deceea5dc3c', '68deddec31eaa', '68e4f339b4a0b'}
            if hashes and hashes.issubset(demo_hashes):
                log(f"Detected known demo images for {chapter_url} (hashes: {hashes})")
                return []  # Skip downloading demo content
            
            # Fallback: if very low uniqueness ratio, likely demo content
            if len(filtered_urls) > 10 and len(hashes) <= len(filtered_urls) * 0.3:
                log(f"Detected placeholder images for {chapter_url} ({len(filtered_urls)} URLs, {len(hashes)} unique hashes)")
                return []  # Skip downloading placeholder content
        
        return filtered_urls
        
    except Exception as e:
        error(f"Failed to fetch images for chapter {chapter_url}: {e}")
        return []


def build_series_url(data):
    """Build series URL from series data."""
    return f"{BASE_URL}/title/{data.get('hash_id')}-{data.get('slug')}"
def main():
    """Main entry point for the scraper."""
    # Create session
    session = get_session()
    
    # Extract all series
    series_data, _ = extract_series_urls(session, 1)
    
    for series in series_data:
        series_url = series['series_url']
        title = extract_series_title(session, series_url)
        if not title:
            error(f"Could not extract title for {series_url}")
            continue
        
        log(f"Processing series: {title}")
        
        # Extract chapters
        chapters = extract_chapter_urls(session, series_url)
        if not chapters:
            log(f"No chapters found for {title}")
            continue
        
        # Get the primary group from the first chapter
        first_group = chapters[0].get('group', '')
        series_tag = f'[Comix - {first_group}]' if first_group else '[Comix]'
        
        # Prepare series info
        series_info = {
            'clean_title': title,
            'existing_chapters': set(),
            'series_directory': Path(CONFIG['folder']) / f"{title} {series_tag}",
            'padding_width': 3
        }
        
        # Download poster
        poster_data = series.get('poster_data')
        if poster_data:
            download_poster(poster_data, series_info['series_directory'], session)
        
        # Process each chapter
        for chapter in chapters:
            try:
                group = chapter.get('group', '')
                config_copy = CONFIG.copy()
                config_copy['tag'] = f'[Comix - {group}]' if group else '[Comix]'
                process_chapter(session, chapter, series_info, config_copy, extract_image_urls, ALLOWED_DOMAINS, BASE_URL)
            except Exception as e:
                error(f"Failed to process chapter {chapter['url']}: {e}")
                continue


if __name__ == "__main__":
    main()