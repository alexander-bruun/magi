#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Rebuild CBZ with correct image order
# DEPRECATED: Just re-download if CBZ is not right

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://asuracomic.net}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/AsuraScans}"
default_suffix="${default_suffix:-[AsuraScans]}"

# Ensure folder exists
mkdir -p "${folder}"

# Function to normalize chapter numbers in existing CBZ files
# Extract series URLs from listing page
# Returns: series URLs (one per line) and sets global variable is_last_page
extract_series_urls() {
  local page=$1
  local url="${domain}/series?page=${page}"
  log "Fetching series list from page $page..."
  
  local html=$(curl -s "$url")
  
  # Check if this is the last page by looking for disabled "Next" button
  if echo "$html" | grep -q 'pointer-events:\s*none.*Next'; then
    is_last_page=true
  else
    is_last_page=false
  fi
  
  # Match series URLs with the 8-character hex suffix pattern (e.g., series/nano-machine-be19545a)
  # This extracts the actual series grid (15 per page), not the popular sidebar
  echo "$html" | grep -oP 'href="series/[a-z0-9-]+-[a-f0-9]{8}"' | sed 's|href="||; s|"$||; s|^|/|' | sort -u
}

# Extract series title from series page
extract_series_title() {
  local series_url="$1"
  local max_retries=3
  local retry_delay=5
  
  for attempt in $(seq 1 $max_retries); do
    # Extract title from <title> tag and remove " - Asura Scans" suffix
    local title=$(curl -s "${domain}${series_url}" | grep -oP '<title>\K[^<]+' | sed 's/ - Asura Scans$//' | head -n 1)
    
    if [[ -n "$title" ]]; then
      echo "$title"
      return 0
    fi
    
    if [ $attempt -lt $max_retries ]; then
      warn "Failed to extract title (attempt $attempt/$max_retries), retrying in ${retry_delay}s..."
      sleep $retry_delay
    fi
  done
  
  # All retries failed
  return 1
}

# Extract chapter links from series page
extract_chapter_links() {
  local series_url="$1"
  local series_slug=$(echo "$series_url" | grep -oP '/series/\K.*')
  curl -s "${domain}${series_url}" | \
    grep -o "${series_slug}/chapter/[0-9]*" | \
    sort -u | \
    awk -F'/' '{print $NF, $0}' | \
    sort -n | \
    cut -d' ' -f2- | \
    sed "s|^|/series/|"
}

# Extract image URLs from chapter page
extract_image_urls() {
  local chapter_url="$1"
  local max_retries=3
  local retry_delay=5
  
  for attempt in $(seq 1 $max_retries); do
    # Extract image URLs from JSON data embedded in the page
    # The page contains escaped JSON with "order" and "url" fields
    # Extract with order number, sort by order, then extract just URLs
    local result
    result=$(curl -s "${domain}${chapter_url}" | \
      grep -oP '\\\"order\\\":\d+,\\\"url\\\":\\\"https://gg\.asuracomic\.net/storage/media/\d+/[^\\]+\.(jpg|webp)\\\"' | \
      grep -oP '\\\"order\\\":\d+,\\\"url\\\":\\\"https://[^"\\]+\.(jpg|webp)\\\"' | \
      awk -F',' '{print $1 "," $2}' | \
      sort -t':' -k2 -n -u | \
      grep -oP 'https://[^"\\]+\.(jpg|webp)' 2>/dev/null || true)
    
    # Only output if we got valid URLs (containing https://)
    if [[ -n "$result" ]]; then
      echo "$result"
      return 0
    fi
    
    if [ $attempt -lt $max_retries ]; then
      warn "Failed to extract images (attempt $attempt/$max_retries), retrying in ${retry_delay}s..."
      sleep $retry_delay
    fi
  done
  
  # All retries failed
  return 1
}

log "Asura Scans → FULL DOWNLOADER"

# Health check: verify the domain is accessible
log "Performing health check on ${domain}..."
health_check "$domain"
success "Health check passed"

# Collect all series URLs
all_series_urls=()
page=1
is_last_page=false

while true; do
  readarray -t page_series < <(extract_series_urls "$page")
  
  # If no series found on this page, we've reached the end
  if [[ ${#page_series[@]} -eq 0 ]]; then
    log "No series found on page $page, stopping."
    break
  fi
  
  all_series_urls+=("${page_series[@]}")
  log "Found ${#page_series[@]} series on page $page"
  
  # Check if we've reached the last page (Next button is disabled)
  if [[ "$is_last_page" == "true" ]]; then
    log "Reached last page (page $page)."
    break
  fi
  
  ((page++))
done

log "Found ${#all_series_urls[@]} series across $page page(s)"

# Process each series
for series_url in "${all_series_urls[@]}"; do
  if [[ -z "$series_url" ]]; then
    continue
  fi

  log "Processing series: ${series_url}"
  
  # Extract series title
  title=$(extract_series_title "$series_url")
  
  if [[ -z "$title" ]]; then
    error "No title → skip"
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
  readarray -t chapter_links < <(extract_chapter_links "$series_url")
  
  if [[ ${#chapter_links[@]} -eq 0 ]]; then
    warn "No chapters found for ${title}, skipping..."
    continue
  fi
  
  log "Found ${#chapter_links[@]} chapters"
  
  # Determine padding width based on max chapter number
  # Get the last chapter in the sorted array (which is the highest)
  last_chapter_link="${chapter_links[-1]}"
  max_chapter_number=$(echo "$last_chapter_link" | grep -oP '/chapter/\K[0-9]+')
  max_chapter_number=$((10#$max_chapter_number))
  
  # Padding width is the number of digits in the max chapter number
  padding_width=${#max_chapter_number}
  log "Max chapter: $max_chapter_number, Padding width: $padding_width"

  # Normalize existing files BEFORE downloading anything
  normalize_chapter_numbers "$series_directory" "$padding_width"

  # Process each chapter
  for chapter_link in "${chapter_links[@]}"; do
    chapter_number=$(echo "$chapter_link" | grep -oP '/chapter/\K[0-9]+')
    
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
    
    # Extract image URLs
    readarray -t image_urls < <(extract_image_urls "$chapter_link")
    
    if [[ ${#image_urls[@]} -eq 0 ]]; then
      log "Chapter $chapter_number does not exist → skipped"
      continue
    fi
    
    log "Downloading → Chapter $chapter_number [${#image_urls[@]} images] → $chapter_name"
    
    if [ "$dry_run" = false ]; then
      mkdir -p "${directory}"
    fi
    
    # Download each image
    img_counter=1
    download_failed=false
    for image_url in "${image_urls[@]}"; do
      if [[ -z "$image_url" ]]; then
        continue
      fi
      
      # Extract file extension
      ext=$(echo "$image_url" | grep -oP '\.(jpg|jpeg|png|webp)' || echo ".webp")
      filename=$(printf "%03d%s" "$img_counter" "$ext")
      
      if [ "$dry_run" = false ]; then
        printf "  [%03d/%03d] %-50s " "$img_counter" "${#image_urls[@]}" "$image_url"
        
        # Check if image exists with HTTP HEAD request. Allow curl to fail without
        # aborting the entire script (set -e is enabled). Treat empty response
        # or 404 as a missing image and skip the chapter.
        http_code=$(curl -s -o /dev/null -w "%{http_code}" "$image_url" || true)

        if [ -z "$http_code" ] || [ "$http_code" = "404" ]; then
          echo -e "\033[1;31mFailed\033[0m"
          error "Image not found or unreachable (http_code=${http_code}): $image_url"
          error "Skipping chapter due to missing image"
          download_failed=true
          break
        fi
        
        if ! curl -s "$image_url" -o "${directory}/${filename}"; then
          echo -e "\033[1;31mFailed\033[0m"
          error "Failed to download ${image_url}"
          download_failed=true
          break
        fi
        
        echo -e "\033[1;32mSuccess\033[0m"
        
        # Convert all images to PNG if enabled
        if [ "$convert_to_png" = true ]; then
          convert_to_png "${directory}/${filename}"
        fi
      fi
      
      ((img_counter++))
    done

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

success "ALL DONE! Every existing chapter is now in → $folder"
