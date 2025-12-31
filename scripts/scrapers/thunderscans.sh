#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://en-thunderscans.com}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/ThunderScans}"
default_suffix="${default_suffix:-[ThunderScans]}"
user_agent="${user_agent:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36}"

# Ensure folder exists
mkdir -p "${folder}"

# Extract series URLs from comics page with pagination
extract_series_urls() {
  local page=$1
  local url="${domain}/comics/?page=${page}"
  
  local html=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" "$url")
  
  # Check if this is the last page by looking for Next button or "No Post Found"
  if ! echo "$html" | grep -q 'href="?page='$((page+1))'"' || echo "$html" | grep -q "No Post Found"; then
    echo "LAST_PAGE" >&2
  fi
  
  # Extract series URLs from the comics grid
  echo "$html" | grep -oP 'href="https://en-thunderscans\.com/comics/[^"]*/"' | sed 's|href="||; s|"$||' | sort -u
}

# Extract series title from series page
extract_series_title() {
  local series_url="$1"
  local title=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" "$series_url" | grep -oP '<title>\K[^<]+' | sed 's/ &#8211; Thunderscans EN$//')
  echo "$title"
}

# Extract chapter links from series page
extract_chapter_links() {
  local series_url="$1"
  curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" "$series_url" | \
    grep -oP 'href="https://en-thunderscans\.com/[^"]*chapter-[0-9]*/"' | \
    sed 's|href="||; s|"$||' | \
    sort -u
}

# Extract image URLs from chapter page
extract_image_urls() {
  local chapter_url="$1"
  local html=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" "$chapter_url")
  
  # Check if chapter is locked
  if echo "$html" | grep -q "This chapter is locked\|lock-container"; then
    log "Chapter is locked, skipping"
    return 1
  fi
  
  # Extract image URLs from JSON data embedded in the page
  local images_json
  images_json=$(echo "$html" | tr -d '\n' | grep -o '"images":\[[^]]*\]' | head -1)
  
  if [[ -z "$images_json" ]]; then
    log "No images JSON found"
    return 1
  fi
  
  # Extract URLs from the JSON array
  echo "$images_json" | sed 's/\\//g' | grep -o 'https://[^"]*\.webp\|https://[^"]*\.jpg\|https://[^"]*\.png'
}

log "Thunder Scans → ALL SERIES DOWNLOADER"

# Health check: verify the domain is accessible
log "Performing health check on ${domain}..."
health_check "$domain"
success "Health check passed"

# Collect all series URLs
all_series_urls=()
page=1
is_last_page=false

while true; do
  log "Fetching series list from page $page..."
  output=$(extract_series_urls "$page" 2>&1)
  stderr_output=$(echo "$output" | grep "LAST_PAGE" || true)
  series_output=$(echo "$output" | grep -v "LAST_PAGE" || true)
  
  readarray -t page_series <<< "$series_output"
  
  # If no series found on this page, we've reached the end
  if [[ ${#page_series[@]} -eq 0 ]]; then
    log "No series found on page $page, stopping."
    break
  fi
  
  all_series_urls+=("${page_series[@]}")
  log "Found ${#page_series[@]} series on page $page"
  
  # Check if we've reached the last page
  if [[ -n "$stderr_output" ]]; then
    log "Reached last page (page $page)."
    break
  fi
  
  ((page++))
done

log "Total series found: ${#all_series_urls[@]}"

for series_url in "${all_series_urls[@]}"; do
  log "Processing series: ${series_url}"
  
  # Extract series title
  title=$(extract_series_title "$series_url")
  
  if [[ -z "$title" ]]; then
    warn "No title for ${series_url}, skipping..."
    continue
  fi
  
  # Clean title for filesystem
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
  readarray -t chapter_links < <(extract_chapter_links "$series_url")
  
  if [[ ${#chapter_links[@]} -eq 0 ]]; then
    warn "No chapters found for ${title}, skipping..."
    continue
  fi
  
  log "Found ${#chapter_links[@]} chapters"

  # Process each chapter
  for chapter_url in "${chapter_links[@]}"; do
    chapter_number=$(echo "$chapter_url" | grep -oP 'chapter-\K\d+')
    
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
    
    # Extract image URLs
    readarray -t img_urls < <(extract_image_urls "$chapter_url")
    
    if [[ ${#img_urls[@]} -eq 0 ]]; then
      log "No images found for chapter $chapter_number → skipped"
      continue
    fi
    
    mkdir -p "${directory}"
    
    # Download each image
    img_counter=0
    download_failed=false
    for image_url in "${img_urls[@]}"; do
      if [ "$dry_run" = false ]; then
        printf "  [%03d] %-50s " "$img_counter" "$image_url"
        
        if ! curl -s -H "User-Agent: $user_agent" "$image_url" -o "${directory}/$(printf "%03d.jpg" "$img_counter")"; then
          echo -e "\033[1;31mFailed\033[0m"
          error "Failed to download ${image_url}"
          download_failed=true
          break
        fi
        
        echo -e "\033[1;32mSuccess\033[0m"
        
        # Convert all images to PNG if enabled
        if [ "$convert_to_png" = true ]; then
          convert_to_png "${directory}/$(printf "%03d.jpg" "$img_counter")" || true
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

success "ALL DONE! Every existing free chapter for all series is now in → $folder"