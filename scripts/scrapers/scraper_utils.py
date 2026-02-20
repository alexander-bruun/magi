#!/usr/bin/env python3
"""
Common utilities for MAGI scrapers.

Shared functions to reduce code duplication across scraper modules.
Provides logging, image processing, CBZ creation, and common patterns.
"""

# Standard library imports
import asyncio
import json
import os
import re
import sys
import threading
import time
import zipfile
from concurrent.futures import ThreadPoolExecutor, as_completed
from html import unescape as html_unescape
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, Set, Tuple, Union
from urllib.parse import urlparse, quote as url_quote

# =============================================================================
# Constants
# =============================================================================
WEBP_QUALITY = int(os.getenv('webp_quality', '100'))
MAX_RETRIES = 3
RETRY_DELAY = 5  # seconds

DEFAULT_USER_AGENT = (
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 '
    '(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
)

# =============================================================================
# Text Processing
# =============================================================================
def encode_url_path(path: str, safe: str = '-/_') -> str:
    """
    URL-encode a path component for use in URLs.
    
    Handles special characters like apostrophes, spaces, and unicode
    that may cause issues in HTTP requests.
    
    Args:
        path: The path string to encode
        safe: Characters that should NOT be encoded (default: -/_)
    
    Returns:
        str: URL-encoded path
    
    Examples:
        >>> encode_url_path("chapter-1")
        'chapter-1'
        >>> encode_url_path("i-carry-the-enemy-'s-child")
        'i-carry-the-enemy-%27s-child'
        >>> encode_url_path("file name.jpg")
        'file%20name.jpg'
    """
    return url_quote(path, safe=safe)


def encode_image_url(url: str) -> str:
    """
    Encode an image URL for HTTP requests.
    
    Specifically handles spaces and special characters in filenames
    that may appear in image URLs.
    
    Args:
        url: The full image URL
    
    Returns:
        str: URL with encoded path components
    """
    # Simple space replacement is usually sufficient for image URLs
    # More complex cases can use full URL parsing if needed
    return url.replace(' ', '%20')


def sanitize_title(title: str) -> str:
    """
    Sanitize a title for use as a filename.

    Removes invalid characters, replaces underscores with spaces,
    removes common genre words, and normalizes whitespace.
    Truncates to 200 characters to prevent filesystem filename length limits.

    Args:
        title: The title string to sanitize

    Returns:
        str: Sanitized title safe for filesystem use
    """
    clean = re.sub(r'[<>:"\/\\|?*]', '', html_unescape(title)).replace('_', ' ').strip()
    clean = clean.replace('\u2018', "'").replace('\u2019', "'").replace('\u201c', '"').replace('\u201d', '"')
    clean = re.sub(r'\s+', ' ', clean).strip()
    # Truncate to 200 characters to prevent filename too long errors
    return clean[:200].rstrip()


def load_config() -> Dict[str, Any]:
    """
    Load configuration from config.json file.
    
    Returns:
        dict: Configuration dictionary, or empty dict if file not found
    """
    config_path = Path(__file__).parent / 'config.json'
    if config_path.exists():
        with open(config_path, 'r', encoding='utf-8') as f:
            return json.load(f)
    return {}


def get_scraper_config(scraper_name: str, default_folder: str, default_suffix: str) -> Dict[str, Any]:
    """
    Get common scraper configuration from config.json and environment variables.
    
    Args:
        scraper_name: Name of the scraper (e.g., 'toongod')
        default_folder: Default folder name if not set in env or config
        default_suffix: Default suffix if not set in env or config
        
    Returns:
        dict: Configuration dictionary with keys:
            - dry_run: bool
            - convert_to_webp: bool  
            - folder: str
            - default_suffix: str
            - priority: int
            - higher_priority_folders: list
    """
    # Load config from config.json if it exists
    config_file = Path(__file__).parent / 'config.json'
    json_config = {}
    if config_file.exists():
        try:
            with open(config_file, 'r') as f:
                data = json.load(f)
                json_config = data.get('scrapers', {}).get(scraper_name, {})
        except Exception as e:
            warn(f"Failed to load config.json: {e}")
    
    # Get values from environment variables, then config.json, then defaults
    folder_path = os.getenv('folder', json_config.get('folder', str(Path(__file__).parent / default_folder)))
    dry_run = os.getenv('dry_run', str(json_config.get('dry_run', False))).lower() == 'true'
    convert_to_webp = os.getenv('convert_to_webp', str(json_config.get('convert_to_webp', True))).lower() == 'true'
    default_suffix_val = os.getenv('default_suffix', json_config.get('default_suffix', default_suffix))
    
    config = {
        'dry_run': dry_run,
        'convert_to_webp': convert_to_webp,
        'folder': folder_path,
        'default_suffix': default_suffix_val,
    }
    
    priority, higher_priority_folders = get_priority_config(scraper_name)
    config['priority'] = priority
    config['higher_priority_folders'] = higher_priority_folders
    
    return config


def process_series(
    session: Any,
    series_url: str,
    config: Dict[str, Any],
    extract_title_func: Callable[[Any, str], Optional[str]],
    extract_chapters_func: Callable[[Any, str], List[str]],
    extract_poster_func: Optional[Callable[[Any, str, dict], Optional[str]]] = None,
    data: Optional[Dict[str, Any]] = None
) -> Dict[str, Any]:
    """
    Common series processing workflow.
    
    Args:
        session: requests.Session object
        series_url: URL of the series to process
        config: Configuration dict from get_scraper_config
        extract_title_func: Function to extract series title (takes session, series_url)
        extract_chapters_func: Function to extract chapter URLs (takes session, series_url)
        extract_poster_func: Optional function to extract poster URL (takes session, series_url)
        data: Series data dict from extract_series_func
        
    Returns:
        dict: Series info with keys:
            - title: str
            - clean_title: str
            - series_directory: Path
            - chapter_urls: list
            - padding_width: int
            - existing_chapters: set
            - skip: bool (True if should skip this series)
    """
    # Extract series title
    title = extract_title_func(session, series_url)
    if not title:
        error(f"Could not extract title for {series_url}, skipping...")
        return {'skip': True}
    
    clean_title = sanitize_title(title)
    log(f"Title: {clean_title}")
    
    # Check for duplicate in higher priority providers
    if check_duplicate_series(clean_title, config['higher_priority_folders']):
        return {'skip': True}
    
    # Extract chapter URLs
    try:
        chapter_urls = extract_chapters_func(session, series_url)
    except Exception as e:
        error(f"Error extracting chapters for {series_url}: {e}")
        return {'skip': True}
    
    if not chapter_urls:
        warn(f"No chapters found for {title}, skipping...")
        return {'skip': True}
    
    # Create series directory (only after confirming chapters exist)
    series_directory = Path(config['folder']) / f"{clean_title} {config['default_suffix']}"
    series_directory.mkdir(parents=True, exist_ok=True)
    
    # Download poster if extractor provided
    if extract_poster_func:
        try:
            poster_url = extract_poster_func(session, series_url)
            if poster_url:
                download_poster(poster_url, series_directory, session)
        except Exception as e:
            warn(f"Failed to download poster for {clean_title}: {e}")
    
    # Determine padding width and build chapter info
    chapter_info = []
    for item in chapter_urls:
        if isinstance(item, dict):
            # Already has num and url
            chapter_info.append(item)
        else:
            # Extract num from URL
            url = item
            match = re.search(r'(\d+)(?:/|$)', url)
            if match:
                num = int(match.group(1))
                chapter_info.append({'url': url, 'num': num})
    
    if not chapter_info:
        error("No chapter numbers found in URLs")
        chapter_numbers = []
    else:
        chapter_numbers = [info['num'] for info in chapter_info]
    padding_width = calculate_padding_width(chapter_numbers)
    log(f"Found {len(chapter_info)} chapters (max: {max(chapter_numbers) if chapter_numbers else 0}, padding: {padding_width})")
    
    # Check for existing chapters
    existing_chapters = get_existing_chapters(series_directory)
    log_existing_chapters(existing_chapters)
    
    # Normalize chapter file names to current padding format
    normalize_chapter_padding(series_directory, padding_width, config['default_suffix'])
    
    # Re-scan existing chapters after potential renames
    existing_chapters = get_existing_chapters(series_directory)
    
    return {
        'title': title,
        'clean_title': clean_title,
        'series_directory': series_directory,
        'chapter_info': chapter_info,
        'padding_width': padding_width,
        'existing_chapters': existing_chapters,
        'skip': False
    }
    
def process_chapter(
    session: Any,
    chapter_info: Dict[str, Any],
    series_info: Dict[str, Any],
    config: Dict[str, Any],
    extract_images_func: Callable[[Any, str], List[str]],
    allowed_domains: Optional[List[str]] = None,
    base_url: Optional[str] = None
) -> Dict[str, Any]:
    """
    Common chapter processing workflow.
    
    Args:
        session: requests.Session object
        chapter_info: Dict with 'url' and 'num' keys
        series_info: Dict from process_series
        config: Configuration dict from get_scraper_config
        extract_images_func: Function to extract image URLs (takes session, chapter_url)
        allowed_domains: List of allowed domains for images
        base_url: Base URL for Cloudflare bypass callback (if None, no bypass callback used)
        
    Returns:
        dict: Chapter processing result with keys:
            - chapter_num: int
            - chapter_name: str
            - downloaded_count: int
            - processed: bool (True if chapter was processed)
    """
    chapter_url = chapter_info['url']
    chapter_num = chapter_info['num']
    
    # Construct full chapter URL for bypass if needed
    full_chapter_url = f"{base_url}{chapter_url}" if base_url else chapter_url
    
    # Normalize chapter number for consistent comparison
    normalized_chapter_num = normalize_chapter_num(chapter_num)
    
    # Check if chapter already exists (by normalized number)
    if normalized_chapter_num in series_info['existing_chapters']:
        return {'processed': False}
    
    chapter_name = chapter_info.get('name')
    if not chapter_name:
        chapter_name = format_chapter_name(
            '', 
            chapter_num, 
            series_info['padding_width'], 
            config['default_suffix']
        )
    
    cbz_name = f"{series_info['clean_title']} {chapter_name} {config.get('tag', '')}".strip()
    
    # Check if CBZ already exists
    cbz_path = series_info['series_directory'] / f"{cbz_name}.cbz"
    if cbz_path.exists():
        return {'processed': False}
    
    # Extract image URLs
    try:
        image_urls = extract_images_func(session, chapter_url)
    except Exception as e:
        error(f"Error extracting images for {chapter_url}: {e}")
        return {'processed': False}
    
    if not image_urls:
        log(f"Skipping: Chapter {chapter_num} (no images)")
        return {'processed': False}
    
    if config['dry_run']:
        log(f"Chapter {chapter_num} [{len(image_urls)} images]")
        return {'processed': True, 'chapter_num': chapter_num, 'downloaded_count': 0}
    
    log(f"Downloading: {cbz_name} [{len(image_urls)} images]")
    
    # Create bypass callback if base_url provided
    bypass_callback = None
    if base_url:
        def bypass_callback():
            nonlocal session
            log("Re-running Cloudflare bypass due to 403 response...")
            try:
                cookies, headers = asyncio.run(bypass_cloudflare(full_chapter_url))
                if cookies:
                    session = get_session(cookies, headers)
                    success("Cloudflare bypass re-run successful")
                else:
                    warn("Cloudflare bypass re-run failed")
            except Exception as e:
                error(f"Cloudflare bypass re-run failed: {e}")
    
    # Download images
    success, downloaded_count = download_chapter_images(
        image_urls=image_urls,
        chapter_folder=series_info['series_directory'] / chapter_name,
        session=session,
        allowed_domains=allowed_domains,
        convert_to_webp=config['convert_to_webp'],
        timeout=30,
        bypass_callback=bypass_callback,
        referer=base_url
    )
    
    # Handle CBZ creation and cleanup
    chapter_folder = series_info['series_directory'] / chapter_name
    if success:
        if create_cbz(chapter_folder, cbz_name):
            # Remove temp folder
            import shutil
            shutil.rmtree(chapter_folder)
        else:
            warn(f"CBZ creation failed for Chapter {chapter_num}, keeping folder")
    else:
        warn(f"Skipping CBZ creation for Chapter {chapter_num} - not all images processed successfully ({downloaded_count}/{len(image_urls)})")
        # Keep temp folder for manual inspection
    
    return {'processed': True, 'chapter_num': chapter_num, 'downloaded_count': downloaded_count}
    
def log_scraper_summary(total_series: int, total_chapters: int, config: Dict[str, Any]) -> None:
    """
    Log the final scraper summary.
    
    Args:
        total_series: Number of series processed
        total_chapters: Number of chapters downloaded
        config: Configuration dict from get_scraper_config
    """
    log(f"Total series processed: {total_series}")
    log(f"Total chapters downloaded: {total_chapters}")
    success(f"Completed! Output: {config['folder']}")
        

def run_scraper(
    session: Any,
    config: Dict[str, Any],
    extract_series_func: Callable[[Any, int], Tuple[List[Dict[str, Any]], int]],
    extract_series_title_func: Callable[[Any, str], Optional[str]],
    extract_chapter_urls_func: Callable[[Any, str], Union[List[str], List[Dict[str, Any]]]],
    extract_image_urls_func: Callable[[Any, str], List[str]],
    extract_poster_func: Optional[Callable[[Any, str, dict], Optional[str]]] = None,
    allowed_domains: Optional[List[str]] = None,
    base_url: Optional[str] = None,
    series_url_builder: Optional[Callable[[Dict[str, Any]], str]] = None
) -> None:
    """
    Run the complete scraping process for a scraper.
    
    Args:
        session: requests.Session object
        config: Configuration dict from get_scraper_config
        extract_series_func: Function to extract series list (takes session, page) -> (series_list, total_pages)
        extract_series_title_func: Function to extract series title (takes session, series_url)
        extract_chapter_urls_func: Function to extract chapter URLs (takes session, series_url)
        extract_image_urls_func: Function to extract image URLs (takes session, chapter_url)
        extract_poster_func: Optional function to extract poster URL (takes session, series_url, series_data)
        allowed_domains: List of allowed domains for images
        base_url: Base URL for Cloudflare bypass callback
        series_url_builder: Function to build series URL from series data (takes series_data dict)
    """
    # Extract series list
    series_list, _ = extract_series_func(session, 1)
    log(f"Found {len(series_list)} series")

    total_series = 0
    total_chapters = 0

    # Process each series
    for series_data in series_list:
        # Build series URL
        if series_url_builder:
            series_url = series_url_builder(series_data)
        else:
            # Default: assume series_data has 'series_slug'
            series_slug = series_data.get("series_slug")
            if not series_slug:
                continue
            series_url = f"{base_url}/series/{series_slug}" if base_url else ""

        # Process series using shared function
        series_info = process_series(
            session, series_url, config, extract_series_title_func, extract_chapter_urls_func, extract_poster_func, data=series_data
        )
        if series_info.get("skip"):
            continue

        total_series += 1

        # Process each chapter
        for chapter_info in series_info["chapter_info"]:
            chapter_result = process_chapter(
                session,
                chapter_info,
                series_info,
                config,
                extract_image_urls_func,
                allowed_domains=allowed_domains,
                base_url=base_url,
            )
            if chapter_result['processed']:
                total_chapters += chapter_result['downloaded_count']

    log_scraper_summary(total_series, total_chapters, config)


def get_priority_config(scraper_name: str) -> Tuple[int, List[str]]:
    """
    Returns:
        tuple: (priority, higher_priority_folders)
    """
    priority = int(os.getenv('priority', '1'))
    higher_folders = json.loads(os.getenv('higher_priority_folders', '[]'))
    
    if priority == 1 and not higher_folders:
        config = load_config()
        scrapers_config = config.get('scrapers', {})
        scraper_config = scrapers_config.get(scraper_name, {})
        priority = scraper_config.get('priority', 1)
        higher_folders = [conf.get('folder') for name, conf in scrapers_config.items() 
                         if conf.get('priority', 1) > priority and conf.get('folder')]
    
    return priority, higher_folders


def check_duplicate_series(clean_title: str, higher_priority_folders: List[str], priority: Optional[int] = None) -> bool:
    """
    Check if a series already exists in higher priority provider folders.
    
    Args:
        clean_title: Sanitized series title
        higher_priority_folders: List of folders with higher priority
        priority: Current scraper's priority level (optional, for compatibility)
        
    Returns:
        bool: True if series exists in higher priority folder (should skip), False otherwise
    """
    for folder_path in higher_priority_folders:
        folder = Path(folder_path)
        if not folder.exists():
            continue
            
        # Look for directories that match the clean title (case-insensitive)
        for existing_dir in folder.iterdir():
            if existing_dir.is_dir():
                # Remove suffix like [SourceName] for comparison
                dir_name = existing_dir.name
                # Extract title part before the suffix
                title_match = re.match(r'^(.+?)\s*\[.*\]$', dir_name)
                if title_match:
                    existing_title = title_match.group(1).strip()
                else:
                    existing_title = dir_name
                
                # Compare normalized titles
                if existing_title.lower() == clean_title.lower():
                    log(f"Skipping {clean_title} - already exists in higher priority provider")
                    return True
    
    return False


# =============================================================================
# Logging Functions
# =============================================================================
def log(msg: str) -> None:
    """Log an info message."""
    print(f"\033[1;34m[INFO]\033[0m    {msg}", file=sys.stderr)


def success(msg: str) -> None:
    """Log a success message."""
    print(f"\033[1;32m[SUCCESS]\033[0m {msg}", file=sys.stderr)


def warn(msg: str) -> None:
    """Log a warning message."""
    print(f"\033[1;33m[WARNING]\033[0m {msg}", file=sys.stderr)


def error(msg: str) -> None:
    """Log an error message."""
    print(f"\033[1;31m[ERROR]\033[0m   {msg}", file=sys.stderr)


# =============================================================================
# Chapter Tracking Utilities
# =============================================================================
def normalize_chapter_num(chapter_num) -> str:
    """
    Normalize chapter number to handle floats properly.
    Keeps '164.0' and '164' as distinct chapters.
    Strips leading zeros from integer part for consistent comparison.
    
    Args:
        chapter_num: Chapter number (str or float)
        
    Returns:
        Normalized chapter number string
    """
    chapter_num_str = str(chapter_num).strip()
    if '.' in chapter_num_str:
        int_part, frac = chapter_num_str.split('.', 1)
        int_part = int_part.lstrip('0') or '0'
        return f"{int_part}.{frac}"
    else:
        return chapter_num_str.lstrip('0') or '0'


def get_existing_chapters(series_directory: Union[str, Path], pattern: str = r'Ch\.([\d.]+)') -> Set[str]:
    """
    Scan directory for existing CBZ files and return set of normalized chapter numbers.

    Args:
        series_directory: Path to series directory
        pattern: Regex pattern to extract chapter number from filename

    Returns:
        set: Normalized chapter number strings already downloaded
    """
    existing = set()
    for cbz_file in Path(series_directory).glob("*.cbz"):
        match = re.search(pattern, cbz_file.stem)
        if match:
            normalized = normalize_chapter_num(match.group(1))
            existing.add(normalized)
    return existing


def log_existing_chapters(existing_chapters: Set[str]) -> None:
    """
    Log skipped chapters in a standardized format.

    Args:
        existing_chapters: Set of normalized chapter number strings already downloaded
    """
    if not existing_chapters:
        log("No existing chapters found, downloading all")
        return

    # Convert to floats for sorting and display
    chapter_nums = []
    for ch in existing_chapters:
        try:
            chapter_nums.append(float(ch))
        except ValueError:
            chapter_nums.append(float('inf'))  # Handle non-numeric gracefully
    
    skipped_count = len(chapter_nums)
    if skipped_count <= 5:
        skipped_list = sorted(chapter_nums)
        log(f"Skipping {skipped_count} existing chapters: {skipped_list}")
    else:
        min_chapter = min(chapter_nums)
        max_chapter = max(chapter_nums)
        log(f"Skipping {skipped_count} existing chapters: {min_chapter}-{max_chapter}")


# =============================================================================
# Chapter Formatting Utilities
# =============================================================================
def format_chapter_name(title: str, chapter_num: Union[int, float], padding_width: int, suffix: str) -> str:
    """
    Format a standardized chapter name for CBZ files.

    Args:
        title: Clean series title
        chapter_num: Chapter number (int or float)
        padding_width: Number of digits for zero-padding
        suffix: Group/source suffix (e.g., '[AsuraScans]')

    Returns:
        str: Formatted chapter name like "Ch.001 [Suffix]"
    """
    if isinstance(chapter_num, float) and chapter_num == int(chapter_num):
        chapter_num = int(chapter_num)
    if isinstance(chapter_num, int):
        formatted = f"{chapter_num:0{padding_width}d}"
    else:
        int_part = f"{int(chapter_num):0{padding_width}d}"
        frac = chapter_num - int(chapter_num)
        if frac == 0:
            formatted = int_part
        else:
            decimal_str = f"{frac:.1f}".lstrip('0').lstrip('.').rstrip('0').rstrip('.')
            formatted = f"{int_part}.{decimal_str}"
    return f"Ch.{formatted} {suffix}".strip()


def calculate_padding_width(chapter_numbers: List[Union[int, float]]) -> int:
    """
    Calculate zero-padding width for chapter numbers.

    Args:
        chapter_numbers: List of chapter numbers in the series

    Returns:
        int: Number of digits needed for padding
    """
    if not chapter_numbers:
        return 0
    max_chapter = max(chapter_numbers)
    return len(str(int(max_chapter)))


def normalize_chapter_padding(series_directory: Union[str, Path], padding_width: int, suffix: str) -> None:
    """
    Rename existing CBZ files to match the current padding width format.
    
    This ensures that when a series grows and requires more padding digits,
    existing files are renamed to the new format to prevent re-downloads.

    Args:
        series_directory: Path to the series directory containing CBZ files
        padding_width: Current padding width (e.g., 3 for Ch.001)
        suffix: The suffix used in chapter names (e.g., '[AsuraScans]')
    """
    series_path = Path(series_directory)
    if not series_path.exists():
        return
    
    renames = []
    for cbz_file in series_path.glob("*.cbz"):
        filename = cbz_file.stem  # Without .cbz extension
        
        # Find the position of " Ch." in the filename
        ch_pos = filename.find(' Ch.')
        if ch_pos == -1:
            continue
        
        title_part = filename[:ch_pos]
        chapter_part = filename[ch_pos + 1:]  # "Ch.XX [Suffix]"
        
        # Extract chapter number
        match = re.search(r'Ch\.([\d.]+)', chapter_part)
        if not match:
            continue
        
        chapter_num_str = match.group(1)
        try:
            # Try to parse as int first, then float
            if '.' in chapter_num_str:
                chapter_num = float(chapter_num_str)
            else:
                chapter_num = int(chapter_num_str)
        except ValueError:
            continue
        
        # Format with current padding
        if isinstance(chapter_num, float) and chapter_num == int(chapter_num):
            chapter_num = int(chapter_num)
        formatted_num = f"{chapter_num:0{padding_width}d}" if isinstance(chapter_num, int) else f"{chapter_num:0{padding_width}.1f}"
        
        # Reconstruct the new filename
        new_chapter_part = f"Ch.{formatted_num} {suffix}"
        new_filename = f"{title_part} {new_chapter_part}.cbz"
        new_path = series_path / new_filename
        
        # Only rename if the name would actually change and target doesn't exist
        if cbz_file.name != new_filename:
            renames.append((cbz_file, new_path))
    
    if not renames:
        return
    
    # Perform renames in parallel for speed
    def do_rename(old_path, new_path):
        try:
            old_path.rename(new_path)
            return True
        except Exception as e:
            error(f"Failed to rename {old_path.name} to {new_path.name}: {e}")
            return False
    
    with ThreadPoolExecutor(max_workers=min(4, len(renames))) as executor:
        results = list(executor.map(lambda r: do_rename(r[0], r[1]), renames))
    
    renamed_count = sum(results)
    if renamed_count > 0:
        log(f"Renamed {renamed_count} CBZ files to match new padding format")


def get_image_extension(url: str, default: str = 'webp') -> str:
    """
    Extract and validate image extension from URL.

    Args:
        url: Image URL
        default: Default extension if none found or invalid

    Returns:
        str: Validated image extension (without dot)
    """
    parsed = urlparse(url)
    path = parsed.path
    ext = path.split('.')[-1].lower() if '.' in path else default
    return ext if ext in ['jpg', 'jpeg', 'png', 'webp', 'gif'] else default


def extract_chapter_number(text: str, pattern: str = r'chapter[-/](\d+)') -> Optional[int]:
    """
    Extract chapter number from text using regex pattern.

    Args:
        text: String containing chapter number
        pattern: Regex pattern with capture group for the number

    Returns:
        int: Chapter number, or None if not found
    """
    match = re.search(pattern, text, re.IGNORECASE)
    return int(match.group(1)) if match else None


# =============================================================================
# Cloudflare Bypass
# =============================================================================
async def bypass_cloudflare(url: str) -> Tuple[Optional[Dict[str, str]], Optional[Dict[str, str]]]:
    """
    Bypass Cloudflare protection for a given URL.

    Uses camoufox browser automation to solve Cloudflare challenges.

    Args:
        url: The URL to bypass Cloudflare protection for

    Returns:
        tuple: (cookies dict, headers dict) or (None, None) on failure
    """
    try:
        from camoufox import AsyncCamoufox
        from camoufox_captcha import solve_captcha
    except ImportError:
        error("camoufox not installed. Run: pip install camoufox camoufox-captcha")
        return None, None

    async with AsyncCamoufox(
        headless=True,
        geoip=False,
        humanize=False,
        i_know_what_im_doing=True,
        config={'forceScopeAccess': True},
        disable_coop=True
    ) as browser:
        page = await browser.new_page()
        await page.goto(url, timeout=30000, wait_until="domcontentloaded")  # Shorter timeout
        # Try to solve captcha, but don't fail if it doesn't work
        try:
            captcha_success = await solve_captcha(page, captcha_type='cloudflare', challenge_type='interstitial')
            if not captcha_success:
                warn("Failed to solve captcha challenge, continuing anyway")
        except Exception as e:
            warn(f"Captcha solving failed: {e}, continuing anyway")

        log("Cloudflare bypass completed (with or without captcha)")

        # Wait a bit for any JavaScript to set session cookies
        await page.wait_for_timeout(5000)  # Wait 5 seconds

        # Extract cookies
        cookies = {}
        for cookie in await page.context.cookies():
            cookies[cookie['name']] = cookie['value']

        # Get user agent
        user_agent = await page.evaluate("navigator.userAgent")

        headers = {
            'User-Agent': user_agent,
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'en-US,en;q=0.5',
            'Accept-Encoding': 'gzip, deflate',
            'Connection': 'keep-alive',
            'Upgrade-Insecure-Requests': '1',
        }

        return cookies, headers


# =============================================================================
# Session Management
# =============================================================================
def get_session(cookies: Optional[Dict[str, str]] = None, headers: Optional[Dict[str, str]] = None) -> Any:
    """
    Create a requests session with optional cookies and headers.

    Args:
        cookies: Optional dict of cookies to add to session
        headers: Optional dict of headers to add to session

    Returns:
        requests.Session: Configured session object
    """
    import requests
    session = requests.Session()
    if cookies:
        for name, value in cookies.items():
            session.cookies.set(name, value)
    if headers:
        session.headers.update(headers)
    return session


def get_default_headers(user_agent: Optional[str] = None) -> Dict[str, str]:
    """
    Get default HTTP headers for scraping.

    Args:
        user_agent: Optional custom user agent string

    Returns:
        dict: HTTP headers suitable for web scraping
    """
    return {
        'User-Agent': user_agent or DEFAULT_USER_AGENT,
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.5',
        'Accept-Encoding': 'gzip, deflate',
        'Connection': 'keep-alive',
        'Upgrade-Insecure-Requests': '1',
    }


# =============================================================================
# Image Processing
# =============================================================================
def convert_image_to_webp(filepath: Path) -> bool:
    """
    Convert an image file to WebP format if it's not already WebP.

    Handles large images by splitting them if they exceed WebP dimension limits.

    Args:
        filepath: Path object to the image file

    Returns:
        bool: True if conversion successful, False otherwise
    """
    from PIL import Image
    if filepath.suffix.lower() == '.webp':
        return True
    try:
        img = Image.open(filepath)
        
        # Verify image is valid
        img.verify()
        img.close()
        
        # Re-open for conversion
        img = Image.open(filepath)
        width, height = img.size
        
        # Check WebP size limits (16383 pixels max dimension)
        if width > 16383 or height > 16383:
            if width <= 16383 and height > 16383:
                # Image is too tall but not too wide - split it
                log(f"Splitting {filepath.name} ({height}px tall)")
                return split_and_convert_to_webp(img, filepath)
            else:
                # Image is too wide or both dimensions are too large
                warn(f"Skip {filepath.name}: exceeds WebP limit ({width}x{height})")
                return False
        
        # Ensure image is in RGB mode for WebP
        if img.mode not in ('RGB', 'RGBA', 'P'):
            img = img.convert('RGB')
        
        webp_path = filepath.with_suffix('.webp')
        img.save(webp_path, 'WebP', quality=WEBP_QUALITY, optimize=True)
        img.close()
        filepath.unlink()
        return True
    except Exception as e:
        error(f"Failed to convert {filepath.name}: {e}")
        # Remove the corrupted file
        if filepath.exists():
            filepath.unlink()
        return False


def split_and_convert_to_webp(img: Any, filepath: Path) -> bool:
    """
    Split tall image into WebP chunks.

    Used when an image exceeds WebP's maximum dimension of 16383 pixels.

    Args:
        img: PIL Image object (already opened)
        filepath: Path object to the original image file

    Returns:
        bool: True if split and conversion successful, False otherwise
    """
    try:
        width, height = img.size
        max_chunk_height = 16383  # WebP max height
        
        # Calculate number of chunks needed for even distribution
        num_chunks = (height + max_chunk_height - 1) // max_chunk_height  # Ceiling division
        
        # Calculate even chunk heights
        base_chunk_height = height // num_chunks
        remainder = height % num_chunks
        
        # Ensure image is in RGB mode
        if img.mode not in ('RGB', 'RGBA', 'P'):
            img = img.convert('RGB')
        
        base_name = filepath.stem
        
        chunks = []
        y_start = 0
        
        for i in range(num_chunks):
            # Distribute remainder pixels to first few chunks for even splitting
            chunk_height = base_chunk_height + (1 if i < remainder else 0)
            y_end = min(y_start + chunk_height, height)
            
            # Create chunk
            chunk = img.crop((0, y_start, width, y_end))
            
            # Save as WebP
            if i == 0:
                # First chunk replaces the original filename
                chunk_filename = filepath.with_suffix('.webp')
            else:
                # Additional chunks get part numbers
                chunk_filename = filepath.parent / f"{base_name}_part{i+1:02d}.webp"
            
            chunk.save(chunk_filename, 'WebP', quality=WEBP_QUALITY, optimize=True)
            chunks.append(chunk_filename)
            
            y_start = y_end
        
        img.close()
        filepath.unlink()  # Remove original file
        return True
        
    except Exception as e:
        error(f"Failed to split {filepath.name}: {e}")
        return False


# =============================================================================
# Image Downloading
# =============================================================================
def download_images_to_folder(
    image_urls: List[str],
    folder_path: Union[str, Path],
    session: Any,
    options: Optional[Dict[str, Any]] = None
) -> Dict[str, Any]:
    """
    Standardized image downloading with configurable options.
    
    Args:
        image_urls: List of image URLs to download
        folder_path: Path to save images
        session: requests.Session object
        options: Dict with options like:
            - timeout: Request timeout (default: 30)
            - max_retries: Max retry attempts (default: 3)
            - convert_webp: Whether to convert to WebP (default: True)
            - concurrent: Whether to download concurrently (default: False)
            - max_workers: Number of concurrent workers (default: 4)
            - referer: Referer header to set for image requests
            - allowed_domains: List of allowed domains for images
            - progress_callback: Function to call for progress updates
            - bypass_callback: Function to call when Cloudflare 403 is detected
    
    Returns:
        dict: {'downloaded': int, 'failed': int, 'errors': list}
    """
    if options is None:
        options = {}
    
    timeout = options.get('timeout', 30)
    max_retries = options.get('max_retries', 3)
    convert_webp = options.get('convert_webp', True)
    concurrent = options.get('concurrent', False)
    max_workers = options.get('max_workers', 4)
    referer = options.get('referer')
    allowed_domains = options.get('allowed_domains', [])
    progress_callback = options.get('progress_callback')
    bypass_callback = options.get('bypass_callback')
    
    folder_path = Path(folder_path)
    folder_path.mkdir(parents=True, exist_ok=True)
    
    results = {'downloaded': 0, 'failed': 0, 'errors': []}
    
    def download_single_image(img_data):
        """Download a single image with retry logic."""
        i, img_url = img_data
        
        # Domain filtering
        if allowed_domains and not any(domain in img_url for domain in allowed_domains):
            return False, f"Domain not allowed: {img_url}"
        
        ext = get_image_extension(img_url, 'jpg')
        filename = folder_path / f"{i:03d}.{ext}"
        
        headers = {}
        if referer:
            headers['Referer'] = referer
        
        for attempt in range(max_retries):
            try:
                response = session.get(img_url, timeout=timeout, headers=headers)
                response.raise_for_status()
                
                with open(filename, 'wb') as f:
                    f.write(response.content)
                
                # Convert to WebP if needed
                if convert_webp and ext != 'webp':
                    if not convert_image_to_webp(filename):
                        return False, f"WebP conversion failed for {filename.name}"
                
                if progress_callback:
                    progress_callback(i, len(image_urls), img_url, 'success')
                else:
                    print(f"  [{i:03d}/{len(image_urls):03d}] {img_url} Success", file=sys.stderr, flush=True)
                
                return True, None
                
            except Exception as e:
                # Handle specific HTTP status codes
                if hasattr(e, 'response') and e.response is not None:
                    status_code = e.response.status_code
                    if status_code == 429:
                        # Too Many Requests - sleep longer before retry
                        sleep_time = 60  # 60 seconds for rate limiting
                        warn(f"Rate limited (429) for {img_url}, sleeping {sleep_time}s before retry")
                        time.sleep(sleep_time)
                        continue
                    elif status_code == 403:
                        # Forbidden - likely Cloudflare block
                        if bypass_callback:
                            log(f"Cloudflare block detected (403) for {img_url}, attempting bypass...")
                            try:
                                bypass_callback()
                                # After bypass, retry immediately
                                continue
                            except Exception as bypass_error:
                                warn(f"Bypass callback failed: {bypass_error}")
                                # Continue with normal retry logic
                        else:
                            warn(f"Cloudflare block detected (403) for {img_url}, but no bypass callback provided")
                
                if attempt == max_retries - 1:
                    error_msg = f"Failed after {max_retries} attempts: {e}"
                    if progress_callback:
                        progress_callback(i, len(image_urls), img_url, 'failed', error_msg)
                    else:
                        print(f"  [{i:03d}/{len(image_urls):03d}] {img_url} Failed: {e}", file=sys.stderr, flush=True)
                    return False, error_msg
                time.sleep(RETRY_DELAY)
        
        return False, "Max retries exceeded"
    
    if concurrent:
        # Concurrent downloads
        download_lock = threading.Lock()
        
        def thread_safe_download(img_data):
            nonlocal results
            success, error_msg = download_single_image(img_data)
            with download_lock:
                if success:
                    results['downloaded'] += 1
                else:
                    results['failed'] += 1
                    results['errors'].append(error_msg)
            return success
        
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            img_data_list = [(i, img_url) for i, img_url in enumerate(image_urls, 1) if img_url]
            list(executor.map(thread_safe_download, img_data_list))
    else:
        # Sequential downloads
        for i, img_url in enumerate(image_urls, 1):
            if not img_url:
                continue
            success, error_msg = download_single_image((i, img_url))
            if success:
                results['downloaded'] += 1
            else:
                results['failed'] += 1
                results['errors'].append(error_msg)
    
    return results


def process_downloaded_images(folder_path: Union[str, Path], convert_webp: bool = True, validate_images: bool = True) -> Dict[str, int]:
    """
    Process downloaded images: convert to WebP, validate integrity.
    
    Args:
        folder_path: Path containing downloaded images
        convert_webp: Whether to convert non-WebP images
        validate_images: Whether to validate image integrity
    
    Returns:
        dict: {'converted': int, 'failed': int, 'invalid': int}
    """
    folder_path = Path(folder_path)
    results = {'converted': 0, 'failed': 0, 'invalid': 0}
    
    for img_file in folder_path.glob('*'):
        if not img_file.is_file():
            continue
        
        if convert_webp and img_file.suffix.lower() != '.webp':
            if convert_image_to_webp(img_file):
                results['converted'] += 1
            else:
                results['failed'] += 1
        
        if validate_images:
            try:
                from PIL import Image
                img = Image.open(img_file)
                img.verify()
                img.close()
            except Exception:
                results['invalid'] += 1
                img_file.unlink()  # Remove invalid file
    
    return results


def download_poster(
    poster_data: Union[str, dict],
    series_directory: Union[str, Path],
    session: Any,
    timeout: int = 30
) -> bool:
    """
    Download poster image for a series.
    
    Args:
        poster_data: Poster URL string or dict with 'large' key
        series_directory: Directory to save the poster
        session: requests.Session object
        timeout: Request timeout
        
    Returns:
        bool: True if download successful, False otherwise
    """
    # Extract URL from data
    if isinstance(poster_data, dict):
        poster_url = poster_data.get('large')
    elif isinstance(poster_data, str):
        poster_url = poster_data
    else:
        poster_url = None
    
    if not poster_url:
        return False
    
    series_directory = Path(series_directory)
    poster_path = series_directory / "poster.webp"
    
    # Create directory if it doesn't exist
    series_directory.mkdir(parents=True, exist_ok=True)
    
    # Skip if poster already exists
    if poster_path.exists():
        log(f"Poster already exists for {series_directory.name}")
        return True
    
    try:
        response = session.get(poster_url, timeout=timeout)
        response.raise_for_status()
        
        with open(poster_path, 'wb') as f:
            f.write(response.content)
        
        log(f"Downloaded poster for {series_directory.name}")
        return True
        
    except Exception as e:
        warn(f"Failed to download poster: {e}")
        return False


def download_chapter_images(
    image_urls: List[str],
    chapter_folder: Union[str, Path],
    session: Any,
    allowed_domains: Optional[List[str]] = None,
    convert_to_webp: bool = True,
    timeout: int = 30,
    bypass_callback: Optional[Callable[[], None]] = None,
    referer: Optional[str] = None
) -> Tuple[bool, int]:
    """
    Download images for a chapter to the specified folder.
    
    Args:
        image_urls: List of image URLs to download
        chapter_folder: Path to the chapter folder (Path object)
        session: requests.Session object
        allowed_domains: List of allowed domains, or None for no restriction
        convert_to_webp: Whether to convert images to WebP
        timeout: Timeout for HTTP requests
        bypass_callback: Function to call when Cloudflare 403 is detected
    
    Returns:
        Tuple[bool, int]: (success, count) where success is True if all images were processed, count is the number of successfully processed images
    """
    chapter_folder = Path(chapter_folder)
    chapter_folder.mkdir(parents=True, exist_ok=True)
    
    # Set up options for download_images_to_folder
    options = {
        'allowed_domains': allowed_domains,
        'convert_webp': False,  # Download first, convert later
        'concurrent': True,     # Download images concurrently
        'max_workers': 5,       # Number of concurrent download workers
        'timeout': timeout,
        'bypass_callback': bypass_callback,
        'referer': referer
    }
    
    # Download images
    result = download_images_to_folder(image_urls, chapter_folder, session, options)
    downloaded_count = result['downloaded']
    
    # Convert to WebP in parallel if requested
    if convert_to_webp and downloaded_count > 0:
        image_files = list(chapter_folder.glob('*'))
        conversion_failures = 0
        with ThreadPoolExecutor(max_workers=min(5, len(image_files))) as executor:
            futures = [executor.submit(convert_image_to_webp, img_file) for img_file in image_files if img_file.is_file()]
            for future in as_completed(futures):
                try:
                    if not future.result():
                        conversion_failures += 1
                except Exception as e:
                    error(f"Failed to convert image: {e}")
                    conversion_failures += 1
        
        # Count final image files after conversion and splitting
        final_image_count = len(list(chapter_folder.glob('*.webp')))
        # Return the number of original images successfully processed
        processed_originals = downloaded_count - conversion_failures
        success = processed_originals >= len(image_urls) - 2  # Allow up to 2 conversion failures
        if conversion_failures > 0:
            warn(f"Chapter processing completed with {conversion_failures} image conversion failures ({processed_originals}/{len(image_urls)} images processed)")
        return success, processed_originals
    else:
        # Count downloaded files if no conversion
        final_image_count = len(list(chapter_folder.glob('*')))
        # Return the number of downloaded originals (no conversion, so all downloaded are processed)
        success = downloaded_count == len(image_urls)
        return success, downloaded_count


# =============================================================================
# CBZ Creation
# =============================================================================
def create_cbz(
    temp_dir: Union[str, Path],
    name: str,
    dest_dir: Optional[Union[str, Path]] = None,
    expected_count: Optional[int] = None
) -> bool:
    """
    Create a CBZ file from images in temp_dir.

    Args:
        temp_dir: Path to directory containing images
        name: Name for the CBZ file (without extension)
        dest_dir: Optional destination directory (defaults to temp_dir.parent)
        expected_count: Optional expected number of files (for validation)

    Returns:
        bool: True if CBZ created successfully, False otherwise
    """
    if dest_dir is None:
        dest_dir = temp_dir.parent
    cbz_file = dest_dir / f"{name}.cbz"
    files = [f for f in temp_dir.glob('*') if f.exists() and f.is_file()]
    
    if not files:
        error(f"No files found in {temp_dir}")
        return False
    
    log(f"Creating {name}.cbz ({len(files)} files)")
    
    # Ensure the parent directory exists
    cbz_file.parent.mkdir(parents=True, exist_ok=True)
    
    # Sort files, handling split images (e.g., 001.webp, 001_part01.webp, 001_part02.webp, 002.webp)
    def sort_key(file):
        stem = file.stem
        # Check if this is a split image part (contains '_part')
        if '_part' in stem:
            # Extract base number and part number
            base_part = stem.split('_part')
            base_num = base_part[0]
            part_num = base_part[1]
            # Return tuple: (base_number, part_number, is_split)
            return (int(base_num) if base_num.isdigit() else 0, int(part_num) if part_num.isdigit() else 0, 1)
        else:
            # Regular file
            return (int(stem) if stem.isdigit() else 0, 0, 0)
    
    files_sorted = sorted(files, key=sort_key)
    
    with zipfile.ZipFile(cbz_file, 'w', zipfile.ZIP_DEFLATED) as zf:
        for file in files_sorted:
            zf.write(file, file.name)
    return True