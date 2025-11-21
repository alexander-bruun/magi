#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://flamecomics.xyz}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/FlameComics}"
default_suffix="${default_suffix:-[FlameComics]}"
user_agent="${user_agent:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36}"

# Ensure folder exists
mkdir -p "${folder}"

# Extract series URLs from browse page
extract_series_urls() {
  local url="${domain}/browse"
  log "Fetching series from browse page..."
  
  local html=$(curl -s -H "User-Agent: $user_agent" "$url")
  
  # Extract series URLs (numeric IDs)
  echo "$html" | grep -oP 'href="/series/\d+"' | sed 's|href="|'$domain'|; s|"$||' | sort -u
}

# Extract series title from series page
extract_series_title() {
  local series_url="$1"
  local title

  local attempts=3
  local delay=5
  for ((i=1; i<=attempts; i++)); do
    if title=$(curl -s -H "User-Agent: $user_agent" "$series_url" | grep -oP '<title>\K[^<]+' | sed 's/ - Flame Comics$//' | head -n 1); then
      if [[ -n "$title" ]]; then
        echo "$title"
        return 0
      fi
    fi
    if [ $i -lt $attempts ]; then
      sleep $delay
    fi
  done
  return 1
}

# Extract chapter links from series page
extract_chapter_links() {
  local series_url="$1"
  local html=$(curl -s -H "User-Agent: $user_agent" "$series_url")
  
  # Extract __NEXT_DATA__ JSON
  local json_data=$(echo "$html" | grep -oP '<script id="__NEXT_DATA__"[^>]*>\K.*?(?=</script>)' | head -n 1)
  
  if [[ -z "$json_data" ]]; then
    error "Could not find __NEXT_DATA__ in $series_url"
    return 1
  fi
  
  # Parse chapters and sort by chapter number
  echo "$json_data" | jq -r '.props.pageProps.chapters | sort_by(.chapter | tonumber) | .[] | "'$domain'/series/\(.series_id)/\(.token)"' 2>/dev/null || true
}

# Extract image URLs from chapter page
extract_image_urls() {
  local chapter_url="$1"
  local urls

  local attempts=3
  local delay=5
  for ((i=1; i<=attempts; i++)); do
    local html=$(curl -s -H "User-Agent: $user_agent" "$chapter_url")
    
    # Extract __NEXT_DATA__ JSON
    local json_data=$(echo "$html" | grep -oP '<script id="__NEXT_DATA__"[^>]*>\K.*?(?=</script>)' | head -n 1)
    
    if [[ -z "$json_data" ]]; then
      if [ $i -lt $attempts ]; then
        sleep $delay
      fi
      continue
    fi
    
    # Parse images from JSON
    urls=$(echo "$json_data" | jq -r '.props.pageProps.chapter as $chapter | $chapter.images | to_entries[] | select(.value.name | contains("commission") | not) | "https://cdn.flamecomics.xyz/uploads/images/series/\($chapter.series_id)/\($chapter.token)/\(.value.name)"' 2>/dev/null || true)
    
    if [[ -n "$urls" ]]; then
      echo "$urls"
      return 0
    fi

    if [ $i -lt $attempts ]; then
      sleep $delay
    fi
  done

  # All retries failed
  return 1
}

echo "Starting Flame Comics scraper..."

log "Flame Comics → FULL DOWNLOADER"

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
all_series_urls=()
readarray -t all_series_urls < <(extract_series_urls)

log "Total series found: ${#all_series_urls[@]}"

# Process each series
for series_url in "${all_series_urls[@]}"; do
  if [[ -z "$series_url" ]]; then
    continue
  fi

  # TEMP: Only process series 99 for testing
  # if [[ "$series_url" != *"series/99" ]]; then
  #   continue
  # fi

  log "Processing series: ${series_url}"
  
  # Extract series title
  title=$(extract_series_title "$series_url")
  
  if [[ -z "$title" ]]; then
    error "Could not extract title for ${series_url}, skipping..."
    continue
  fi
  
  # Clean title for filesystem (also unescape HTML entities like &#x27;)
  clean_title=$(html_unescape "$title" | sed -e 's/[<>:"\/\\|?*]//g' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
  
  log "Series: $title"
  
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
    log "No chapters found for ${title}, skipping..."
    continue
  fi
  
  log "Found ${#chapter_links[@]} chapters"
  
  # Determine padding width based on max chapter number
  # Get the last chapter in the sorted array (which is the highest)
  last_chapter_link="${chapter_links[-1]}"
  # For Flame Comics, chapter number is not in URL, so we need to extract from the page or assume
  # Since we sorted by chapter number, we can count the number of chapters
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
      log "No images found for chapter $chapter_number (may be unavailable or error page), skipping..."
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
          log "  Image not found or unreachable (http_code=${http_code}): $image_url"
          log "  Skipping chapter due to missing image"
          download_failed=true
          break
        fi
        
        if ! curl -s -H "User-Agent: $user_agent" "$image_url" -o "${directory}/${filename}"; then
          echo -e "\033[1;31mFailed\033[0m"
          log "Failed to download ${image_url}"
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
        log "  Cleaning up incomplete chapter directory"
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