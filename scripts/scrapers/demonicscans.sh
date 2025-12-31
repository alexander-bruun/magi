#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/DemonicScans}"
default_suffix="${default_suffix:-[DemonicScans]}"

# Extract series URLs from the translations list page
# Returns: series URLs (one per line) and sets global variable is_last_page
extract_series_urls() {
  local page=$1
  local url="https://demonicscans.org/translationlist.php?page=${page}"
  log "Fetching series list from page $page..."
  
  local html=$(curl -s --max-time 30 "$url")
  
  # Check if this is the last page by looking for disabled "Next" button
  if echo "$html" | grep -q 'pointer-events:\s*none.*Next'; then
    is_last_page=true
  else
    is_last_page=false
  fi
  
  # Extract series URLs
  echo "$html" | grep -oP 'href="/manga/[^\"]+"' | sed 's|href="||; s|"||' | sort -u
}

# Extract series title from series page
extract_series_title() {
  local series_url="$1"
  local title

  local attempts=3
  local delay=5
  for ((i=1; i<=attempts; i++)); do
    if title=$(bash -c "
      t=\$(curl -s --max-time 30 'https://demonicscans.org${series_url}' | grep -oP '<title>\K[^<]+' | sed 's/ - Demonic Scans\$//' | head -n 1)
      if [[ -n \"\$t\" ]]; then
        echo \"\$t\"
      else
        exit 1
      fi
    " 2>/dev/null); then
      echo "$title"
      return 0
    fi
    if [ $i -lt $attempts ]; then
      sleep $delay
    fi
  done
  return 1
}

# Extract chapter URLs for a given manga
extract_chapter_urls() {
    manga_url="$1"
    full_url="https://demonicscans.org$manga_url"
    raw_html=$(curl -s --max-time 30 "$full_url" | tr -d '\0')

    echo "$raw_html" | \
    grep -a -oP 'href="/chaptered.php\?manga=[^&]+&chapter=[^"]+"' | \
    sed 's|href="||; s|"||' | \
    sort -V | uniq
}

# Extract image URLs for a given chapter
extract_image_urls() {
    chapter_url="$1"
    full_url="https://demonicscans.org$chapter_url"
    raw_html=$(curl -s -L --max-time 30 "$full_url" | tr -d '\0')

    echo "$raw_html" | \
    grep -oP 'https?://[^"]*\.(jpg|jpeg|png|webp)' | \
    grep -E '(demoniclibs\.com|mangareadon\.org)' | \
    awk '!seen[$0]++'
}

echo "Starting Demonic Scans scraper..."

log "Demonic Scans → FULL DOWNLOADER"

# Health check: verify the domain is accessible
log "Performing health check on https://demonicscans.org..."
health_check_response=$(curl -s -o /dev/null -w "%{http_code}" "https://demonicscans.org")

if [ "$health_check_response" != "200" ]; then
  error "Health check failed. https://demonicscans.org returned HTTP ${health_check_response}"
  error "The site may be down or unreachable. Exiting."
  exit 1
fi

success "Health check passed. Site is accessible."

# Ensure folder exists
mkdir -p "${folder}"

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
  log "Processing series: ${series_url}"

  title=$(extract_series_title "$series_url")

  if [[ -z "$title" ]]; then
    error "Could not extract title for ${series_url}, skipping..."
    continue
  fi

  clean_title=$(html_unescape "$title" | sed -e 's/[<>:"\/\\|?*]//g' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')

  log "Title → $clean_title"

  # Create series directory
  series_directory="${folder}/${clean_title} ${default_suffix}"
  mkdir -p "$series_directory"

  # Extract chapter URLs for the current series
  readarray -t chapter_urls < <(extract_chapter_urls "$series_url")

  if [[ ${#chapter_urls[@]} -eq 0 ]]; then
    warn "No chapters found for ${title}, skipping..."
    continue
  fi
  
  log "Found ${#chapter_urls[@]} chapters"  # Determine padding width based on max chapter number
  # Get the last chapter in the sorted array (which is the highest)
  last_chapter_url="${chapter_urls[-1]}"
  max_chapter_number=$(echo "$last_chapter_url" | sed 's/.*chapter=\([0-9]*\).*/\1/')
  max_chapter_number=$((10#$max_chapter_number))
  
  # Padding width is the number of digits in the max chapter number
  padding_width=${#max_chapter_number}
  log "Max chapter: $max_chapter_number, Padding width: $padding_width"

  for chapter_url in "${chapter_urls[@]}"; do
    log "Processing chapter: ${chapter_url}"

    # Extract chapter number
    chapter_num=$(echo "$chapter_url" | sed 's/.*chapter=\([0-9]*\).*/\1/')

    # Force decimal (fix octal issue)
    decnum=$((10#$chapter_num))
    formatted_chapter_number=$(printf "%0${padding_width}d" "$decnum")

    chapter_name="${clean_title} Ch.${formatted_chapter_number} ${default_suffix}"

    # Check if CBZ exists
    shopt -s nullglob
    matches=( "${series_directory}"/*"Ch.${formatted_chapter_number}"*.cbz )
    if [ "${#matches[@]}" -gt 0 ]; then
      shopt -u nullglob
      # CBZ exists, skip download
      continue
    fi
    shopt -u nullglob

    log "Trying → Chapter $decnum"

    readarray -t image_urls < <(extract_image_urls "$chapter_url")

    if [[ ${#image_urls[@]} -eq 0 ]]; then
      log "Chapter $decnum does not exist → skipped"
      continue
    fi

    log "Downloading → Chapter $decnum [${#image_urls[@]} images] → $chapter_name"

    # Create chapter directory
    chapter_folder="${series_directory}/${chapter_name}"
    mkdir -p "$chapter_folder"

    # Change to chapter directory
    cd "$chapter_folder"

    # Download and convert images
    img_counter=1
    for i in "${!image_urls[@]}"; do
      img_url="${image_urls[$i]}"
      if [[ -z "$img_url" ]] || ([[ "$img_url" != *demoniclibs.com* ]] && [[ "$img_url" != *mangareadon.org* ]]); then
        continue
      fi
      # URL encode spaces
      img_url=$(printf '%s' "$img_url" | sed 's/ /%20/g')
      ext="${img_url##*.}"
      filename=$(printf "%03d.%s" $((i+1)) "$ext")
      if [ "$dry_run" = false ]; then
        printf "  [%03d/%03d] %-50s " "$img_counter" "${#image_urls[@]}" "$img_url"
        if ! curl -s --max-time 30 "$img_url" -o "$filename"; then
          echo -e "\033[1;31mFailed\033[0m"
          continue
        fi
        echo -e "\033[1;32mSuccess\033[0m"
        if [ "$convert_to_png" = true ] && [[ "$ext" != "png" ]]; then
          convert_to_png "$filename"
        fi
      fi
      ((img_counter++))
    done

    if [ "$dry_run" = false ]; then
      # Create CBZ
      create_cbz "${chapter_folder}" "${chapter_name}"
      # Return to original directory
      cd - > /dev/null
      rm -rf "${chapter_folder}"
    fi

  done

done

success "ALL DONE! Every existing chapter is now in → $folder"
