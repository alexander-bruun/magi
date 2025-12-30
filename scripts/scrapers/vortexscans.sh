#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://vortexscans.org}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/VortexScans}"
default_suffix="${default_suffix:-[VortexScans]}"
user_agent="${user_agent:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36}"
cookie_file="${cookie_file:-$(cd "$(dirname "$0")" && pwd)/cookies.txt}"

# Ensure folder exists
mkdir -p "${folder}"

# Initialize cookies by visiting the main page
# This sets the vShield cookie required to bypass rate limiting.
# The vShield cookie is a session cookie that VortexScans uses to identify legitimate users.
# To get the vShield cookie manually:
# 1. Open your browser and navigate to https://vortexscans.org
# 2. Open Developer Tools (F12), go to the Network tab
# 3. Visit a chapter page, e.g., https://vortexscans.org/series/murim-psychopath/chapter-0
# 4. In the Network tab, find the request to the chapter page, click on it
# 5. In the Headers section, scroll to Request Headers, find the Cookie header
# 6. Look for the vShield cookie value (it might be something like vShield=abc123...)
# 7. Copy the entire cookie string and save it to cookies.txt in Netscape format:
#    vortexscans.org\tFALSE\t/\tFALSE\t0\tvShield\t<cookie_value>
# Alternatively, the script does this automatically by visiting the main page first.
init_cookies() {
  log "Initializing cookies..."
  curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" -c "$cookie_file" -o /dev/null "$domain"
}

# Extract series slugs from API
extract_series_urls() {
  local url="https://api.vortexscans.org/api/query"
  log "Fetching series from API..."
  
  local json=$(curl -s -b "$cookie_file" -c "$cookie_file" "$url")
  
  echo "$json" | jq -r '.posts[].slug' 2>/dev/null || echo "$json" | grep -oP '"slug":"[^"]*"' | sed 's/"slug":"//' | sed 's/"$//' | grep -v '^chapter-' | sort -u
}

# Extract series title from API
extract_series_title() {
  local series_slug="$1"
  local url="https://api.vortexscans.org/api/query?slug=${series_slug}"
  local json=$(curl -s -b "$cookie_file" -c "$cookie_file" "$url")
  
  echo "$json" | jq -r '.posts[] | select(.slug == "'$series_slug'") | .postTitle' 2>/dev/null
}

# Extract chapter links by generating from 0 to max chapter number from API
extract_chapter_links() {
  local series_slug="$1"
  local url="https://api.vortexscans.org/api/query?slug=${series_slug}"
  local json=$(curl -s -b "$cookie_file" -c "$cookie_file" "$url")
  
  local max_chapter=$(echo "$json" | jq -r '.posts[] | select(.slug == "'$series_slug'") | .chapters[].number' 2>/dev/null | sort -n | tail -1)
  
  if [[ -z "$max_chapter" || "$max_chapter" == "null" ]]; then
    max_chapter=0
  fi
  
  for ((i=0; i<=max_chapter; i++)); do
    echo "chapter-$i"
  done
}

log "Vortex Scans → ALL SERIES DOWNLOADER"

init_cookies

# Get all series slugs
readarray -t series_slugs < <(extract_series_urls)

log "Found ${#series_slugs[@]} series to process"

for series_slug in "${series_slugs[@]}"; do
  log "Processing series: ${series_slug}"
  
  # Extract series title
  title=$(extract_series_title "$series_slug")
  
  if [[ -z "$title" ]]; then
    warn "No title for ${series_slug}, skipping..."
    continue
  fi
  
  # Clean title for filesystem (also unescape HTML entities like &#x27;)
  clean_title=$(html_unescape "$title" | sed -e 's/[<>:"\/\\|?*]//g' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
  
  log "Title → $clean_title"
  
  # Check if directory already exists with different suffix
  existing_directory=$(find "${folder}" -maxdepth 1 -type d -name "${clean_title}*" 2>/dev/null | head -n 1 || true)

  if [ -n "$existing_directory" ]; then
    series_directory="$existing_directory"
  else
    series_directory="${folder}/${clean_title} ${default_suffix}"
  fi

  # Extract all chapter links
  readarray -t chapter_links < <(extract_chapter_links "$series_slug")
  
  if [[ ${#chapter_links[@]} -eq 0 ]]; then
    warn "No chapters found for ${title}, skipping..."
    continue
  fi
  
  log "Found ${#chapter_links[@]} chapters"
  
  # Determine padding width based on max chapter number
  # Get the last chapter in the sorted array (which is the highest)
  last_chapter_link="${chapter_links[-1]}"
  max_chapter_number=$(echo "$last_chapter_link" | grep -oP 'chapter-\K\d+')
  max_chapter_number=$((10#$max_chapter_number))
  
  # Padding width is the number of digits in the max chapter number
  padding_width=${#max_chapter_number}
  log "Max chapter: $max_chapter_number, Padding width: $padding_width"

  # Normalize existing files BEFORE downloading anything
  normalize_chapter_numbers "$series_directory" "$padding_width"

  # Process each chapter
  for chapter_link in "${chapter_links[@]}"; do
    chapter_slug="$chapter_link"
    chapter_number=$(echo "$chapter_slug" | grep -oP 'chapter-\K\d+')
    
    if [[ ! "$chapter_number" =~ ^[0-9]+$ ]]; then
      continue
    fi

    # Force decimal (fix octal issue)
    decnum=$((10#$chapter_number))
    formatted_chapter_number=$(printf "%0${padding_width}d" "$decnum")
    chapter_name="${clean_title} Ch.${formatted_chapter_number} ${default_suffix}"
    directory="${series_directory}/${chapter_name}"

    # Check if CBZ exists
    shopt -s nullglob
    matches=( "${series_directory}"/*"Ch.${formatted_chapter_number}"*.cbz )
    if [ "${#matches[@]}" -gt 0 ]; then
      shopt -u nullglob
      # CBZ exists, skip download
      continue
    fi
    shopt -u nullglob

    log "Trying → Chapter $chapter_number"
    
    # Extract series_slug and token from chapter page
    page_url="https://vortexscans.org/series/${series_slug}/${chapter_slug}"
    html=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" -b "$cookie_file" -c "$cookie_file" "$page_url")
    
    if [[ -z "$html" ]]; then
      log "Failed to fetch chapter page for chapter $chapter_number → skipped"
      continue
    fi
    
    # Check for rate limiting
    if echo "$html" | grep -q "Rate Limited"; then
      log "Rate limited for chapter $chapter_number → skipped"
      continue
    fi
    
    # Extract img src from HTML
    img_src=$(echo "$html" | grep -o '<img[^>]*src="https://storage\.vexmanga\.com/public//upload/series/[^"]*\.webp"' | head -1 | sed 's/.*src="//; s/".*//')
    
    if [[ -z "$img_src" ]]; then
      log "No img src found for chapter $chapter_number → skipped"
      continue
    fi
    
    # Extract series_slug and token from img_src
    # img_src: https://storage.vexmanga.com/public//upload/series/{series_slug}/{token}/01.webp
    chapter_series_slug=$(echo "$img_src" | sed 's|https://storage\.vexmanga\.com/public//upload/series/\([^/]*\)/.*|\1|')
    token=$(echo "$img_src" | sed 's|https://storage\.vexmanga\.com/public//upload/series/[^/]*/\([^/]*\)/.*|\1|')
    
    if [[ -z "$chapter_series_slug" || -z "$token" ]]; then
      log "Failed to extract series_slug or token for chapter $chapter_number → skipped"
      continue
    fi
    
    log "Extracted series_slug: $chapter_series_slug, token: $token"
    
    mkdir -p "${directory}"
    
    # Download each image
    img_counter=1
    download_failed=false
    while true; do
      # Generate image URL (using the token from the example)
      i=$(printf "%02d" "$img_counter")
      image_url="https://storage.vexmanga.com/public//upload/series/${chapter_series_slug}/${token}/${i}.webp"
      
      # Check if image exists with HTTP HEAD request
      http_code=$(curl -s -H "User-Agent: $user_agent" -b "$cookie_file" -o /dev/null -w "%{http_code}" "$image_url" || true)

      if [ -z "$http_code" ] || [ "$http_code" = "404" ]; then
        # No more images
        break
      fi
      
      if [ "$dry_run" = false ]; then
        printf "  [%03d] %-50s " "$img_counter" "$image_url"
        
        if ! curl -s -H "User-Agent: $user_agent" -b "$cookie_file" "$image_url" -o "${directory}/$(printf "%03d.webp" "$img_counter")"; then
          echo -e "\033[1;31mFailed\033[0m"
          error "Failed to download ${image_url}"
          download_failed=true
          break
        fi
        
        echo -e "\033[1;32mSuccess\033[0m"
        
        # Convert all images to PNG if enabled
        if [ "$convert_to_png" = true ]; then
          convert_to_png "${directory}/$(printf "%03d.webp" "$img_counter")"
        fi
      else
        # In dry run, just log
        log "  Would download: $image_url"
      fi
      
      ((img_counter++))
      
      # Safety limit
      if [ "$img_counter" -gt 999 ]; then
        break
      fi
    done
    
    # Adjust img_counter to number of images
    num_images=$((img_counter - 1))
    
    if [ "$num_images" -eq 0 ]; then
      log "Chapter $chapter_number does not exist → skipped"
      continue
    fi
    
    log "Downloaded → Chapter $chapter_number [$num_images images] → $chapter_name"

    if [ "$dry_run" = false ]; then
      if [ "$download_failed" = true ]; then
        log "Incomplete → skipped"
        rm -rf "${directory}"
      else
        create_cbz "${directory}" "${chapter_name}"
        rm -rf "${directory}"
      fi
    fi
  done
done

# Disable error trap for clean exit
trap - ERR

success "ALL DONE! Every existing chapter for all series is now in → $folder"