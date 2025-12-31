#!/bin/bash

# Set UTF-8 locale to handle Unicode characters properly
export LC_ALL=C.UTF-8
export LANG=C.UTF-8
export PERL_UNICODE=SDA

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert WebP images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://lhtranslation.net}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/LHTranslation}"
default_suffix="${default_suffix:-[LHTranslation]}"

# Ensure folder exists
mkdir -p "${folder}"

# Extract series URLs from listing page, handling load more
extract_series_urls() {
  local url="${domain}/manga/"
  log "Fetching series from listing page..."
  
  local all_series=()
  local page=1
  local has_more=true
  
  while [ "$has_more" = true ]; do
    local html
    if [ "$page" -eq 1 ]; then
      # First page: direct fetch
      html=$(curl -s "$url")
    else
      # Subsequent pages: load more via AJAX
      local data="action=madara_load_more&page=$page&template=madara-core%2Fcontent%2Fcontent-archive&vars%5Bpaged%5D=$page&vars%5Borderby%5D=meta_value_num&vars%5Btemplate%5D=archive&vars%5Bsidebar%5D=right&vars%5Bpost_type%5D=wp-manga&vars%5Bpost_status%5D=publish&vars%5Bmeta_key%5D=_latest_update&vars%5Border%5D=desc&vars%5Bmeta_query%5D%5Brelation%5D=AND&vars%5Bmanga_archives_item_layout%5D=default"
      html=$(curl -s -X POST "${domain}/wp-admin/admin-ajax.php" \
        -H "Content-Type: application/x-www-form-urlencoded; charset=UTF-8" \
        -H "X-Requested-With: XMLHttpRequest" \
        -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36" \
        --data "$data")
    fi
    
    # Extract series URLs from this page
    local page_series=($(echo "$html" | grep -oP 'href="https://lhtranslation\.net/manga/[^"]*/"' | sed 's|href="||; s|"$||' | grep -v '/chapter-' | grep -v '/feed/' | sort -u))
    
    if [ ${#page_series[@]} -eq 0 ]; then
      has_more=false
    else
      all_series+=("${page_series[@]}")
      log "Found ${#page_series[@]} series on page $page"
      ((page++))
    fi
  done
  
  # Output all unique series
  printf '%s\n' "${all_series[@]}" | sort -u
}

# Extract series title from series page
extract_series_title() {
  local series_url="$1"
  local title

  local attempts=3
  local delay=5
  for ((i=1; i<=attempts; i++)); do
    title=$(curl -s --compressed "$series_url" 2>/dev/null | iconv -f utf-8 -t utf-8 -c 2>/dev/null | grep -oP '<title>\K[^<]+' 2>/dev/null | sed 's/ &#8211; LHTranslation$//' | head -n 1)
    if [[ -n "$title" ]]; then
      echo "$title"
      return 0
    fi
    if [ $i -lt $attempts ]; then
      sleep $delay
    fi
  done
  
  return 1
}

# Extract chapter URLs from series page
extract_chapter_urls() {
  local series_url="$1"
  local ajax_url="${series_url}ajax/chapters/"
  local html=$(curl -s -X POST "$ajax_url")
  
  # Extract chapter URLs from the AJAX response
  echo "$html" | grep -oP 'href="https://lhtranslation\.net/manga/[^"]*chapter-[^"]*/"' | sed 's|href="||; s|"$||' | sort -u
}

# Extract chapter title from chapter URL
extract_chapter_title() {
  local chapter_url="$1"
  local chapter_num=$(echo "$chapter_url" | grep -oP 'chapter-\K[^/]+')
  echo "Chapter $chapter_num"
}

# Download chapter images
download_chapter() {
  local chapter_url="$1"
  local output_dir="$2"
  local html=$(curl -s "$chapter_url")
  
  # Extract image URLs - look for data-src in img tags, handling newlines
  local image_urls=$(echo "$html" | tr '\n' ' ' | grep -oP 'data-src="\s*\Khttps://lhtranslation\.net/wp-content/uploads/WP-manga/data/[^"]*\.(jpg|jpeg|png|webp)' | sort -u)
  
  if [[ -z "$image_urls" ]]; then
    return 1
  fi
  
  local img_counter=1
  echo "$image_urls" | while read -r image_url; do
    if [[ -z "$image_url" ]]; then
      continue
    fi
    
    # Extract file extension
    ext=$(echo "$image_url" | grep -oP '\.(jpg|jpeg|png|webp)$' || echo ".jpg")
    filename=$(printf "%03d%s" "$img_counter" "$ext")
    
    if [ "$dry_run" = false ]; then
      printf "  [%03d] %-50s " "$img_counter" "$image_url"
      
      if ! curl -s "$image_url" -o "${output_dir}/${filename}"; then
        echo -e "\033[1;31mFailed\033[0m"
        return 1
      fi
      
      echo -e "\033[1;32mSuccess\033[0m"
      
      # Convert to PNG if enabled
      if [ "$convert_to_png" = true ]; then
        convert_to_png "${output_dir}/${filename}"
      fi
    fi
    
    ((img_counter++))
  done
  
  return 0
}

log "LHTranslation → FULL DOWNLOADER"

# Health check: verify the domain is accessible
log "Performing health check on ${domain}..."
health_check_response=$(curl -s -L -o /dev/null -w "%{http_code}" "${domain}" | tail -1)
if [ "$health_check_response" != "200" ]; then
  error "Health check failed. ${domain} returned HTTP ${health_check_response}"
  error "The site may be down or unreachable. Exiting."
  exit 1
fi
success "Health check passed"

# Collect all series URLs
log "Collecting series URLs..."
readarray -t all_series_urls < <(extract_series_urls)
log "Found ${#all_series_urls[@]} series"

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
  
  # Check if directory already exists
  existing_directory=$(find "${folder}" -maxdepth 1 -type d -name "${clean_title}*" 2>/dev/null | head -n 1 || true)

  if [ -n "$existing_directory" ]; then
    series_directory="$existing_directory"
  else
    series_directory="${folder}/${clean_title}"
  fi

  # Extract all chapter URLs
  readarray -t chapter_urls < <(extract_chapter_urls "$series_url")
  
  if [[ ${#chapter_urls[@]} -eq 0 ]]; then
    warn "No chapters found for ${title}, skipping..."
    continue
  fi
  
  log "Found ${#chapter_urls[@]} chapters"
  
  # Process each chapter
  for chapter_url in "${chapter_urls[@]}"; do
    chapter_title=$(extract_chapter_title "$chapter_url")
    chapter_name="${clean_title} ${chapter_title} ${default_suffix}"
    directory="${series_directory}/${chapter_name}"

    # Check if CBZ exists
    shopt -s nullglob
    matches=( "${series_directory}"/*"${chapter_title}"*.cbz )
    if [ "${#matches[@]}" -gt 0 ]; then
      shopt -u nullglob
      continue
    fi
    shopt -u nullglob

    log "Trying → ${chapter_title}"
    
    if [ "$dry_run" = false ]; then
      mkdir -p "${directory}"
    fi
    
    # Download chapter
    if download_chapter "$chapter_url" "${directory}"; then
      if [ "$dry_run" = false ]; then
        create_cbz "${directory}" "${chapter_name}"
        rm -rf "${directory}"
      fi
      log "Downloaded → ${chapter_title}"
    else
      log "Failed → ${chapter_title}"
      if [ "$dry_run" = false ]; then
        rm -rf "${directory}"
      fi
    fi
  done
done

# Disable error trap for clean exit
trap - ERR

success "ALL DONE! Every existing chapter is now in → $folder"