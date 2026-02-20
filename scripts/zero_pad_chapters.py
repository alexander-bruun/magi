import os
import re
import sys
from pathlib import Path
from collections import defaultdict

def extract_chapter_number(filename):
    match = re.search(r'Ch\.([\d.]+)', filename)
    if match:
        return float(match.group(1))
    return None

def calculate_padding_width(chapter_numbers):
    if not chapter_numbers:
        return 0
    max_chapter = max(chapter_numbers)
    return len(str(int(max_chapter)))

def process_series(cbz_files):
    chapter_numbers = []
    for cbz_file in cbz_files:
        chapter = extract_chapter_number(cbz_file.name)
        if chapter is not None:
            chapter_numbers.append(chapter)

    if not chapter_numbers:
        return

    width = calculate_padding_width(chapter_numbers)

    for cbz_file in cbz_files:
        match = re.search(r'Ch\.([\d.]+)', cbz_file.name)
        if match:
            chapter_str = match.group(1)
            if '.' in chapter_str:
                parts = chapter_str.split('.')
                int_part = int(parts[0])
                frac_part = '.'.join(parts[1:])
                new_chapter_str = f"{int_part:0{width}d}.{frac_part}"
            else:
                chapter_number = int(chapter_str)
                new_chapter_str = f"{chapter_number:0{width}d}"
            new_name = cbz_file.name.replace(f"Ch.{chapter_str}", f"Ch.{new_chapter_str}")
            new_path = cbz_file.with_name(new_name)
            if new_path != cbz_file:
                print(f"Renamed: {cbz_file.name} -> {new_name}")
                cbz_file.rename(new_path)

def main():
    if len(sys.argv) != 2:
        print("Usage: python zero_pad_chapters.py <directory>")
        sys.exit(1)

    directory = Path(sys.argv[1])
    if not directory.is_dir():
        print(f"Error: {directory} is not a directory")
        sys.exit(1)

    # Find all CBZ files recursively
    all_cbz = list(directory.rglob("*.cbz"))
    if not all_cbz:
        print("No CBZ files found in the directory")
        return

    # Group CBZ files by their immediate parent directory (series)
    series_groups = defaultdict(list)
    for cbz in all_cbz:
        series_groups[cbz.parent].append(cbz)

    # Process each series
    for series_dir, cbz_files in series_groups.items():
        process_series(cbz_files)

if __name__ == "__main__":
    main()
