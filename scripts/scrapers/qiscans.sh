#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://qiscans.org}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/QiScans}"
default_suffix="${default_suffix:-[QiScans]}"
user_agent="${user_agent:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36}"
cookie_file="${cookie_file:-$(cd "$(dirname "$0")" && pwd)/cookies.txt}"
api_cache_file="$(dirname "$cookie_file")/qiscans.json"
original_vshield=$(grep '_1__vShield_v' "$cookie_file" | awk '{print $7}')

# Ensure folder exists
mkdir -p "${folder}"

# Initialize cookies by visiting the main page
# This sets the vShield cookie required to bypass rate limiting.
# The vShield cookie is a session cookie that QiScans uses to identify legitimate users.
# Note: The vShield cookie value is dynamic and changes with each session/request.
# The script will update it during execution but restore the original value at the end.
# To get the vShield cookie manually:
# 1. Open your browser and navigate to https://qiscans.org
# 2. Open Developer Tools (F12), go to the Network tab
# 3. Visit a chapter page, e.g., https://qiscans.org/series/some-series/chapter-0
# 4. In the Network tab, find the request to the chapter page, click on it
# 5. In the Headers section, scroll to Request Headers, find the Cookie header
# 6. Look for the vShield cookie value (it might be something like vShield=abc123...)
# 7. Copy the entire cookie string and save it to cookies.txt in Netscape format:
#    qiscans.org\tFALSE\t/\tFALSE\t0\tvShield\t<cookie_value>
# Alternatively, the script does this automatically by visiting the main page first.
init_cookies() {
  log "Initializing cookies..."
  curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" -c "$cookie_file" -o /dev/null "$domain"
}

# Extract series slugs from API
extract_series_urls() {
  echo "$all_json" | jq -r '.posts[].slug' 2>/dev/null || echo "$all_json" | grep -oP '"slug":"[^"]*"' | sed 's/"slug":"//' | sed 's/"$//' | grep -v '^chapter-' | sort -u
}

# Extract series title from API
extract_series_title() {
  local series_slug="$1"
  echo "$all_json" | jq -r '.posts[] | select(.slug == "'$series_slug'") | .postTitle' 2>/dev/null
}

# Extract chapter links from series page JSON
extract_chapter_links() {
  local series_slug="$1"
  local series_url="https://qiscans.org/series/${series_slug}"
  local html=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.5" -H "Referer: https://qiscans.org/" -b "$cookie_file" -c "$cookie_file" "$series_url")
  
  local tmp_html=$(echo "$html" | tr -d '\n')
  grep -o '\\"slug\\":\\"chapter-[^"]*\\"' <<< "$tmp_html" | sed 's/\\"slug\\":\\"//' | sed 's/\\"//' | sort | uniq
}

log "Qi Scans → ALL SERIES DOWNLOADER"

init_cookies

if [[ ! -f "$api_cache_file" ]]; then
  log "Fetching all series data..."
  all_json=$(curl -s -H "User-Agent: $user_agent" -H "Accept: application/json" -H "Accept-Language: en-US,en;q=0.9" -H "Origin: https://qiscans.org" -H "Referer: https://qiscans.org/" -b "$cookie_file" -c "$cookie_file" "https://api.qiscans.org/api/query?page=1&perPage=99999")
  echo "$all_json" > "$api_cache_file"
else
  log "Loading series data from cache..."
  all_json=$(cat "$api_cache_file")
fi

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
  
  # Skip novels
  if [[ "$title" == *"[Novel]"* ]]; then
    log "Skipping novel: $title"
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

  # Process each chapter
  for chapter_link in "${chapter_links[@]}"; do
    chapter_slug="$chapter_link"
    chapter_number=$(echo "$chapter_slug" | grep -oP 'chapter-\K\d+')
    
    if [[ ! "$chapter_number" =~ ^[0-9]+$ ]]; then
      continue
    fi

    chapter_name="${clean_title} Ch.${chapter_number} ${default_suffix}"
    directory="${series_directory}/${chapter_name}"

    # Check if CBZ exists
    shopt -s nullglob
    matches=( "${series_directory}"/*"Ch.${chapter_number}"*.cbz )
    if [ "${#matches[@]}" -gt 0 ]; then
      shopt -u nullglob
      # CBZ exists, skip download
      continue
    fi
    shopt -u nullglob

    log "Trying → Chapter $chapter_number"
    
    # Extract series_slug and token from chapter page
    page_url="https://qiscans.org/series/${series_slug}/${chapter_slug}"
    log "Fetching images from: $page_url"
    html=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" -b "$cookie_file" -c "$cookie_file" "$page_url")
    
    if [[ -z "$html" ]]; then
      log "Failed to fetch chapter page for chapter $chapter_number → skipped"
      continue
    fi
    
    # Check for premium chapter
    if echo "$html" | grep -q "This premium chapter is waiting to be unlocked."; then
      log "Chapter $chapter_number is premium, skipping"
      continue
    fi
    
    # Check for rate limiting
    if echo "$html" | grep -q "Rate Limited"; then
      log "Rate limited for chapter $chapter_number → skipped"
      continue
    fi
    
    # Extract img tags with data-image-index
    tmp_html=$(mktemp)
    echo "$html" | tr -d '\n' > "$tmp_html"
    img_urls=$(grep -o 'https://media\.qiscans\.org/file/qiscans/upload/series/[^"]*\.webp' "$tmp_html" | sed 's|/file/qiscans||' | sort -V | uniq)
    rm "$tmp_html"
    
    if [[ -z "$img_urls" ]]; then
      log "No img URLs found for chapter $chapter_number → skipped"
      continue
    fi
    
    # Take the first URL for extraction
    first_url=$(echo "$img_urls" | head -1)
    
    # Extract series_slug and token from first_url
    chapter_series_slug=$(echo "$first_url" | sed 's|.*/series/\([^/]*\)/.*|\1|')
    token=$(echo "$first_url" | sed 's|.*/\([^/]*\)/[^/]*$|\1|')
    
    # Extract first_number from first_url
    first_number=$(echo "$first_url" | sed 's|.*/\([0-9]*\)\.\(webp\|jpg\)$|\1|')
    
    # Ensure first_number is a valid number
    if [[ ! "$first_number" =~ ^[0-9]+$ ]]; then
      first_number=0
    fi
    
    if [[ -z "$chapter_series_slug" || -z "$token" || -z "$first_number" ]]; then
      if [[ -z "$chapter_series_slug" ]]; then log "Failed to extract series_slug from img_url: $first_url"; fi
      if [[ -z "$token" ]]; then log "Failed to extract token from img_url: $first_url"; fi
      if [[ -z "$first_number" ]]; then log "Failed to extract first_number from img_url: $first_url"; fi
      log "Skipping chapter $chapter_number"
      continue
    fi
    
    log "Extracted series_slug: $chapter_series_slug, token: $token, first_number: $first_number"
    
    mkdir -p "${directory}"
    
    # Download each image
    img_counter=0
    download_failed=false
    for image_url in $img_urls; do
      # URL encode spaces
      image_url=$(printf '%s' "$image_url" | sed 's/ /%20/g')
      
      # Extract extension
      ext=$(echo "$image_url" | sed 's/.*\.\(webp\|jpg\)$/\1/')
      
      if [ "$dry_run" = false ]; then
        printf "  [%03d] %-50s " "$img_counter" "$image_url"
        
        if ! curl -s -H "User-Agent: $user_agent" -b "$cookie_file" "$image_url" -o "${directory}/$(printf "%03d.%s" "$img_counter" "$ext")"; then
          echo -e "\033[1;31mFailed\033[0m"
          error "Failed to download ${image_url}"
          download_failed=true
          break
        fi
        
        echo -e "\033[1;32mSuccess\033[0m"
        
        # Convert all images to PNG if enabled
        if [ "$convert_to_png" = true ]; then
          convert_to_png "${directory}/$(printf "%03d.%s" "$img_counter" "$ext")" || true
        fi
      else
        # In dry run, just log
        log "  Would download: $image_url"
      fi
      
      img_counter=$((img_counter + 1))
    done
    
    num_images=$img_counter
    
    if [ "$num_images" -le 0 ]; then
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

# Restore original vShield cookie
sed -i "s/\(_1__vShield_v\t\)[^\t]*/\1$original_vshield/" "$cookie_file"

success "ALL DONE! Every existing chapter for all series is now in → $folder"