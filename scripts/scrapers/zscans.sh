#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://zscans.com}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/ZScans}"
default_suffix="${default_suffix:-[ZScans]}"
user_agent="${user_agent:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36}"

# Ensure folder exists
mkdir -p "${folder}"

# Extract series URLs from comics page
extract_series_urls() {
  local url="${domain}/comics"
  log "Fetching series from comics page..."
  
  local html=$(curl -s -H "User-Agent: $user_agent" "$url")
  
  # Extract series slugs from quoted strings in the HTML that match slug patterns
  # Filter out common HTML/CSS terms and keep only valid slugs
  echo "$html" | grep -oP '"[a-z0-9-]+(?:-[a-z0-9-]+)*"' | \
    grep -v -E '(css|js|png|jpg|webp|svg|ico|woff|ttf|eot|px|app|button|alert|all|canonical|charset|content|data|div|form|head|html|http|icon|img|input|link|meta|nav|none|page|path|rel|script|span|style|text|title|type|url|var|view|xml|lang|language|container|bookmark|horizontal|font-weight-bold|hooper-list|hooper-next|hooper-prev|hooper-track|action|comedy|drama|fantasy|horror|isekai|manga|manhua|manhwa|mystery|romance|supernatural|historical|completed|dropped|ongoing|hiatus|new|one-shot|martial-arts|reincarnation)' | \
    sed 's/"//g' | \
    grep -E '^[a-z][a-z0-9]*(-[a-z0-9]+)+$' | \
    sort -u
}

# Extract series title from series page
extract_series_title() {
  local series_slug="$1"
  local max_retries=3
  local retry_delay=5
  
  for attempt in $(seq 1 $max_retries); do
    local url="${domain}/comics/${series_slug}"
    local html=$(curl -s -H "User-Agent: $user_agent" "$url")
    
    # Try to extract title from JavaScript data first
    local title=$(echo "$html" | grep -oP 'name:"[^"]*"' | sed 's/name:"//' | sed 's/"$//' | head -n 1)
    
    if [[ -z "$title" ]]; then
      # Fallback to page title
      title=$(echo "$html" | grep -oP '<title>\K[^<]+' | sed 's/ • Zero Scans$//' | sed 's/^Read //' | sed 's/ with up to date chapters!$//' | head -n 1)
    fi
    
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
  local series_slug="$1"
  local url="${domain}/comics/${series_slug}"
  local html=$(curl -s -H "User-Agent: $user_agent" "$url")
  
  # Extract chapter count and first chapter ID
  local chapter_count=$(echo "$html" | grep -oP 'chapters_count:\d+' | sed 's/chapters_count://')
  local first_chapter_id=$(echo "$html" | grep -oP 'first_chapter:\[\{[^}]*,id:\d+\}' | grep -oP 'id:\d+' | sed 's/id://')
  
  if [[ -z "$chapter_count" || -z "$first_chapter_id" ]]; then
    error "Could not find chapter information in $url"
    return 1
  fi
  
  # Generate chapter URLs assuming sequential IDs starting from first_chapter_id
  # This may not be accurate for all series, but it's the best we can do with static scraping
  for ((i=0; i<chapter_count; i++)); do
    local chapter_id=$((first_chapter_id + i))
    echo "${domain}/comics/${series_slug}/${chapter_id}"
  done
}

# Extract image URLs from chapter page
extract_image_urls() {
  local chapter_url="$1"
  local max_retries=3
  local retry_delay=5

  for attempt in $(seq 1 $max_retries); do
    local html=$(curl -s -H "User-Agent: $user_agent" "$chapter_url")
    
    # Extract image URLs from the JavaScript data
    # Look for high_quality or good_quality arrays
    local images_data=$(echo "$html" | grep -oP '(high_quality|good_quality):\[.*?\]' | head -n 1)
    
    if [[ -n "$images_data" ]]; then
      # Extract URLs from the array and unescape \u002F to /
      echo "$images_data" | sed 's/\\u002F/\//g' | grep -oP '"[^"]*"' | sed 's/"//g' | while read -r url; do
        if [[ "$url" =~ ^https:// ]]; then
          echo "$url"
        fi
      done
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

# Decode common HTML entities in titles (e.g. &#x27; -> ', &amp; -> &)
echo "Starting Z Scans scraper..."

log "Z Scans → FULL DOWNLOADER"

# Health check: verify the domain is accessible
log "Performing health check on ${domain}..."
health_check_response=$(curl -s -H "User-Agent: $user_agent" -o /dev/null -w "%{http_code}" "${domain}")

if [ "$health_check_response" != "200" ]; then
  error "Health check failed. ${domain} returned HTTP ${health_check_response}"
  error "The site may be down or unreachable. Exiting."
  exit 1
fi

success "Health check passed. Site is accessible."

# Collect all series URLs
all_series_slugs=()
readarray -t all_series_slugs < <(extract_series_urls)

log "Total series found: ${#all_series_slugs[@]}"

# Process each series
for series_slug in "${all_series_slugs[@]}"; do
  if [[ -z "$series_slug" ]]; then
    continue
  fi

  log "Processing series: ${series_slug}"
  
  # Extract series title
  title=$(extract_series_title "$series_slug")
  
  if [[ -z "$title" ]]; then
    warn "Could not extract title for ${series_slug}, skipping..."
    continue
  fi
  
  # Clean title for filesystem (also unescape HTML entities like &#x27;)
  clean_title=$(html_unescape "$title" | sed -e 's/[<>:"\/\\|?*]//g' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
  
  log "Series: $title"
  
  # Check if directory already exists with different suffix
  existing_directory=$(find "${folder}" -maxdepth 1 -type d -name "${clean_title}*" 2>/dev/null | head -n 1)

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
  # For Z Scans, chapters seem to be numbered sequentially
  max_chapter_number=${#chapter_links[@]}
  
  # Padding width is the number of digits in the max chapter number
  padding_width=${#max_chapter_number}
  log "Max chapter: $max_chapter_number, Padding width: $padding_width"

  # Normalize existing files BEFORE downloading anything
  normalize_chapter_numbers "$series_directory" "$padding_width"

  # Process each chapter
  chapter_index=1
  for chapter_link in "${chapter_links[@]}"; do
    chapter_number=$chapter_index
    
    if [[ ! "$chapter_number" =~ ^[0-9]+$ ]]; then
      continue
    fi

    # Force decimal
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
      ((chapter_index++))
      continue
    fi
    shopt -u nullglob

    log "Downloading chapter $chapter_number: $chapter_name"
    
    # Extract image URLs
    log "Chapter link: $chapter_link"
    readarray -t image_urls < <(extract_image_urls "$chapter_link")
    
    if [[ ${#image_urls[@]} -eq 0 ]]; then
      warn "No images found for chapter $chapter_number (may be unavailable or error page), skipping..."
      ((chapter_index++))
      continue
    fi
    
    log "Found ${#image_urls[@]} images"
    
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
      ext=$(echo "$image_url" | grep -oP '\.(jpg|jpeg|png|webp)' || echo ".png")
      filename=$(printf "%03d%s" "$img_counter" "$ext")
      
      if [ "$dry_run" = false ]; then
        printf "  [%03d/%03d] %-50s " "$img_counter" "${#image_urls[@]}" "$image_url"
        
        # Check if image exists with HTTP HEAD request
        http_code=$(curl -s -H "User-Agent: $user_agent" -o /dev/null -w "%{http_code}" "$image_url" || true)

        if [ -z "$http_code" ] || [ "$http_code" = "404" ]; then
          echo -e "\033[1;31mFailed\033[0m"
          echo "  Image not found or unreachable (http_code=${http_code}): $image_url"
          echo "  Skipping chapter due to missing image"
          download_failed=true
          break
        fi
        
        if ! curl -s -H "User-Agent: $user_agent" "$image_url" -o "${directory}/${filename}"; then
          echo -e "\033[1;31mFailed\033[0m"
          echo "Failed to download ${image_url}"
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
        echo "  Cleaning up incomplete chapter directory"
        rm -rf "${directory}"
      else
        create_cbz "${directory}" "${chapter_name}"
        rm -rf "${directory}"
      fi
    fi
    
    ((chapter_index++))
  done
done

# Disable error trap for clean exit
trap - ERR

success "ALL DONE! Every existing chapter is now in → $folder"