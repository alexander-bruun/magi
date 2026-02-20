#!/usr/bin/env python3
"""
Script to check .cbz files in /mnt/g for missing image numbers.
Deletes .cbz files that have gaps in their image sequence.
"""

import os
import re
import zipfile
from pathlib import Path

def extract_image_numbers(cbz_path):
    """Extract image numbers from a CBZ file."""
    numbers = set()
    try:
        with zipfile.ZipFile(cbz_path, 'r') as zf:
            for filename in zf.namelist():
                # Match patterns like 001.jpg, 002.webp, 1.png, etc.
                match = re.match(r'^(\d+)\.', filename)
                if match:
                    numbers.add(int(match.group(1)))
    except Exception as e:
        print(f"Error reading {cbz_path}: {e}")
        return None
    return sorted(numbers)

def has_missing_numbers(numbers):
    """Check if the sequence has missing numbers."""
    if not numbers:
        return True  # Empty is considered missing
    min_num = min(numbers)
    max_num = max(numbers)
    expected = set(range(min_num, max_num + 1))
    return expected != set(numbers)

def main():
    """Main function."""
    import subprocess
    
    base_path = '/mnt/g/DemonicScans'
    if not os.path.exists(base_path):
        print("Path /mnt/g does not exist")
        return

    # Use find command to get all .cbz files, excluding recycle bin
    try:
        result = subprocess.run(['find', base_path, '-name', '*.cbz', '-type', 'f', '-not', '-path', '*/$RECYCLE.BIN/*', '-not', '-path', '*/recycle.bin/*'], 
                              capture_output=True, text=True)
        cbz_files = result.stdout.strip().split('\n') if result.stdout.strip() else []
    except Exception as e:
        print(f"Error running find: {e}")
        return

    print(f"Found {len(cbz_files)} .cbz files")

    to_delete = []

    for cbz_file in cbz_files:
        if not cbz_file:
            continue
        numbers = extract_image_numbers(cbz_file)
        if numbers is None:
            continue  # Skip if error
        if has_missing_numbers(numbers):
            to_delete.append(cbz_file)
            print(f"Deleting: {cbz_file} (numbers: {numbers})")
            try:
                os.remove(cbz_file)
                print(f"Deleted: {cbz_file}")
            except Exception as e:
                print(f"Error deleting {cbz_file}: {e}")
        else:
            print(f"OK: {cbz_file}")

    print(f"\nProcessed {len(cbz_files)} files, deleted {len(to_delete)} files")

if __name__ == "__main__":
    main()