#!/usr/bin/env python3
"""
Common utilities for MAGI scrapers.

Shared functions to reduce code duplication across scraper modules.
Provides logging, image processing, CBZ creation, and common patterns.
"""

# Standard library imports
import os
import re
import sys
import zipfile
from html import unescape as html_unescape
from pathlib import Path
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
def encode_url_path(path, safe='-/_'):
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


def encode_image_url(url):
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


def sanitize_title(title):
    """
    Sanitize a title for use as a filename.

    Removes invalid characters, replaces underscores with spaces,
    and normalizes whitespace.

    Args:
        title: The title string to sanitize

    Returns:
        str: Sanitized title safe for filesystem use
    """
    clean = re.sub(r'[<>:"\/\\|?*]', '', html_unescape(title)).replace('_', ' ').strip()
    return re.sub(r'\s+', ' ', clean)


# =============================================================================
# Logging Functions
# =============================================================================
def log(msg):
    """Log an info message."""
    print(f"\033[1;34m[INFO]\033[0m    {msg}", file=sys.stderr)


def success(msg):
    """Log a success message."""
    print(f"\033[1;32m[SUCCESS]\033[0m {msg}", file=sys.stderr)


def warn(msg):
    """Log a warning message."""
    print(f"\033[1;33m[WARNING]\033[0m {msg}", file=sys.stderr)


def error(msg):
    """Log an error message."""
    print(f"\033[1;31m[ERROR]\033[0m   {msg}", file=sys.stderr)


# =============================================================================
# Chapter Tracking Utilities
# =============================================================================
def get_existing_chapters(series_directory, pattern=r'Ch\.([\d.]+)'):
    """
    Scan directory for existing CBZ files and return set of chapter numbers.

    Args:
        series_directory: Path to series directory
        pattern: Regex pattern to extract chapter number from filename

    Returns:
        set: Chapter numbers (as floats) already downloaded
    """
    existing = set()
    for cbz_file in Path(series_directory).glob("*.cbz"):
        match = re.search(pattern, cbz_file.stem)
        if match:
            existing.add(float(match.group(1)))
    return existing


def log_existing_chapters(existing_chapters):
    """
    Log skipped chapters in a standardized format.

    Args:
        existing_chapters: Set of chapter numbers already downloaded
    """
    if not existing_chapters:
        log("No existing chapters found, downloading all")
        return

    skipped_count = len(existing_chapters)
    if skipped_count <= 5:
        skipped_list = sorted(existing_chapters)
        log(f"Skipping {skipped_count} existing chapters: {skipped_list}")
    else:
        min_chapter = min(existing_chapters)
        max_chapter = max(existing_chapters)
        log(f"Skipping {skipped_count} existing chapters: {min_chapter}-{max_chapter}")


# =============================================================================
# Chapter Formatting Utilities
# =============================================================================
def format_chapter_name(title, chapter_num, padding_width, suffix):
    """
    Format a standardized chapter name for CBZ files.

    Args:
        title: Clean series title
        chapter_num: Chapter number (int or float)
        padding_width: Number of digits for zero-padding
        suffix: Group/source suffix (e.g., '[AsuraScans]')

    Returns:
        str: Formatted chapter name like "Title Ch.001 [Suffix]"
    """
    if isinstance(chapter_num, float) and chapter_num == int(chapter_num):
        chapter_num = int(chapter_num)
    formatted = f"{chapter_num:0{padding_width}d}" if isinstance(chapter_num, int) else f"{chapter_num:0{padding_width}.1f}"
    return f"{title} Ch.{formatted} {suffix}"


def calculate_padding_width(max_chapter):
    """
    Calculate zero-padding width for chapter numbers.

    Args:
        max_chapter: Maximum chapter number in series

    Returns:
        int: Number of digits needed for padding
    """
    return len(str(int(max_chapter)))


def get_image_extension(url, default='webp'):
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


def extract_chapter_number(text, pattern=r'chapter-(\d+)'):
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
async def bypass_cloudflare(url):
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
        await page.goto(url)
        captcha_success = await solve_captcha(page, captcha_type='cloudflare', challenge_type='interstitial')
        if not captcha_success:
            error("Failed to solve captcha challenge")
            return None, None

        log("Cloudflare bypass successful")

        # Wait a bit for any JavaScript to set session cookies
        await page.wait_for_timeout(2000)

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
def get_session(cookies=None, headers=None):
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
        session.cookies.update(cookies)
    if headers:
        session.headers.update(headers)
    return session


def get_default_headers(user_agent=None):
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
def convert_to_webp(filepath):
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


def split_and_convert_to_webp(img, filepath):
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
# CBZ Creation
# =============================================================================
def create_cbz(temp_dir, name, dest_dir=None, expected_count=None):
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