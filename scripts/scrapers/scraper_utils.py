#!/usr/bin/env python3
"""
Common utilities for MAGI scrapers.
Shared functions to reduce code duplication across scraper modules.
"""

import re
import sys
import zipfile
import os
from pathlib import Path
from html import unescape as html_unescape

# Configuration
WEBP_QUALITY = int(os.getenv('webp_quality', '100'))

# Text processing
def sanitize_title(title):
    """Sanitize a title for use as a filename. Removes invalid characters, replaces underscores, and normalizes whitespace."""
    clean = re.sub(r'[<>:"\/\\|?*]', '', html_unescape(title)).replace('_', ' ').strip()
    return re.sub(r'\s+', ' ', clean)

# Logging functions
def log(msg):
    """Log an info message."""
    print(f"\033[1;34m[INFO]\033[0m    {msg}", file=sys.stderr)

def success(msg):
    """Log a success message."""
    print(f"\033[1;32m[SUCCESS]\033[0m  {msg}", file=sys.stderr)

def warn(msg):
    """Log a warning message."""
    print(f"\033[1;33m[WARNING]\033[0m {msg}", file=sys.stderr)

def error(msg):
    """Log an error message."""
    print(f"\033[1;31m[ERROR]\033[0m   {msg}", file=sys.stderr)

# Cloudflare bypass
async def bypass_cloudflare(url):
    """Bypass Cloudflare protection for a given URL."""
    try:
        from camoufox import AsyncCamoufox
        from camoufox_captcha import solve_captcha
    except ImportError:
        error("camoufox not installed. Run: pip install camoufox camoufox-captcha")
        return None, None

    async with AsyncCamoufox(
        headless=True,
        geoip=True,
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

def get_session(cookies=None, headers=None):
    """Create a requests session with optional cookies and headers."""
    import requests
    session = requests.Session()
    if cookies:
        session.cookies.update(cookies)
    if headers:
        session.headers.update(headers)
    return session

# Image processing
def convert_to_webp(filepath):
    """Convert an image file to WebP format if it's not already WebP."""
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
    """Split tall image into WebP chunks."""
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

# CBZ creation
def create_cbz(temp_dir, name, dest_dir=None, expected_count=None):
    """Create a CBZ file from images in temp_dir."""
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

# Common scraper configuration
DEFAULT_USER_AGENT = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'

def get_default_headers(user_agent=None):
    """Get default HTTP headers for scraping."""
    return {
        'User-Agent': user_agent or DEFAULT_USER_AGENT,
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.5',
        'Accept-Encoding': 'gzip, deflate',
        'Connection': 'keep-alive',
        'Upgrade-Insecure-Requests': '1',
    }