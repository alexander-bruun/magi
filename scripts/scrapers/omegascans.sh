#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

IFS=$'\n'

# Dry run mode (set to true to enable, false to disable)
dry_run="${dry_run:-false}"

# Convert images to PNG for better comic reader compatibility
convert_to_png="${convert_to_png:-true}"

# Global variables - can be overridden from environment
domain="${domain:-https://omegascans.org}"
api="${api:-https://api.omegascans.org}"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/OmegaScans}"
default_suffix="${default_suffix:-[OmegaScans]}"

# Ensure folder exists
mkdir -p "${folder}"

# Function to normalize chapter numbers in existing CBZ files
# Fetch series data into memory (avoid temporary files)
series_response=$(curl -s "${api}/query/?page=1&perPage=99999999")
if ! echo "$series_response" | jq empty 2>/dev/null; then
  error "Failed to fetch valid series JSON"
  exit 1
fi

log "Omega Scans → FULL DOWNLOADER"

for series in $(echo "$series_response" | jq -c '.data[]'); do
  id=$(echo "$series" | jq -r '.id')
  title=$(echo "$series" | jq -r '.title')

  clean_title=$(echo "$title" | sed -e 's/[<>:"\/\\|?*]//g')

  series_slug=$(echo "$series" | jq -r '.series_slug')
  thumbnail=$(echo "$series" | jq -r '.thumbnail')
  series_type=$(echo "$series" | jq -r '.series_type')
  
  if [ "${series_type}" = "Novel" ]; then
    continue
  fi

  log "Processing series: $title (ID: $id)"

  existing_directory=$(find "${folder}" -maxdepth 1 -type d -name "${clean_title}*" 2>/dev/null || true)

  if [ -n "$existing_directory" ]; then
    series_directory="$existing_directory"
  else
    series_directory="${folder}/${clean_title} ${default_suffix}"
  fi

  # Fetch chapters data into memory (avoid temporary files)
  chapters_response=$(curl -s "${api}/chapter/query?page=1&perPage=99999999&series_id=${id}")

  # Debug: check if chapters_response is valid JSON
  if ! echo "$chapters_response" | jq empty 2>/dev/null; then
    error "Failed to fetch valid chapters JSON for series '$title'"
    continue
  fi

  # Try to extract chapter numbers that appear after 'Ch.' or 'Chapter '
  max_chapter_number=$(echo "$chapters_response" | jq -r '.data[].chapter_name' \
    | grep -oP '(?i)(?<=ch\.|chapter )[0-9]+' \
    | sort -n \
    | tail -n 1)

  # Fallback: if nothing found with the above, extract any numeric tokens and take the max
  if [ -z "$max_chapter_number" ]; then
    max_chapter_number=$(echo "$chapters_response" | jq -r '.data[].chapter_name' \
      | grep -oP '[0-9]+' \
      | sort -n \
      | tail -n 1)
  fi

  if [ -z "$max_chapter_number" ]; then
    max_chapter_number=0
  fi

  padding_width=${#max_chapter_number}
  if [ "$padding_width" -lt 1 ]; then
    padding_width=1
  fi

  # Normalize existing files BEFORE downloading anything
  normalize_chapter_numbers "$series_directory" "$padding_width"

  # Loop through chapters
  for chapter in $(echo "$chapters_response" | jq -c '.data[]'); do
    chapter_name_raw=$(echo "$chapter" | jq -r '.chapter_name' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]\{2,\}/ /g')

    # Skip seasons and decimal chapters (e.g., 12.5)
    if echo "$chapter_name_raw" | grep -qi 'season' || echo "$chapter_name_raw" | grep -qP '\d+\.\d+'; then
      continue
    fi

    chapter_number=$(echo "$chapter_name_raw" | grep -oP '\d+' | head -n 1 | tr -d '\n')
    if [[ ! "$chapter_number" =~ ^[0-9]+$ ]]; then
      chapter_number="0"
    fi

    # Force decimal (fix octal issue)
    decnum=$((10#$chapter_number))
    formatted_chapter_number=$(printf "%0${padding_width}d" "$decnum")

    chapter_name="${clean_title} Ch.${formatted_chapter_number} ${default_suffix}"
    directory="${series_directory}/${chapter_name}"

    # If any existing CBZ matches this formatted chapter number, skip downloading
    shopt -s nullglob
    matches=( "${series_directory}"/*"${formatted_chapter_number}"*.cbz )
    if [ "${#matches[@]}" -gt 0 ]; then
      shopt -u nullglob
      continue
    fi
    shopt -u nullglob

    # Debug: validate chapter JSON before parsing
    if ! echo "$chapter" | jq empty 2>/dev/null; then
      error "Invalid JSON for chapter: $chapter"
      continue
    fi

    chapter_slug=$(echo "$chapter" | jq -r '.chapter_slug')
    chapter_url="${domain}/series/${series_slug}/${chapter_slug}"
    chapter_content=$(curl -s "$chapter_url")
    
    if ! [[ $chapter_content =~ "This chapter is premium!" ]]; then
      log "Downloading: $directory"
      if [ "$dry_run" = false ]; then
        mkdir -p "${directory}"
      fi

      api_urls_src=$(echo "$chapter_content" | grep -oP '(?<=src=")https://api.omegascans.org/uploads/series/[^"]+' | uniq)
      media_urls_src=$(echo "$chapter_content" | grep -oP '(?<=src=")https://media.omegascans.org/file/[^"]+' | uniq)

      # Debug: print number of URLs found
      api_count=$(echo "$api_urls_src" | wc -l)
      media_count=$(echo "$media_urls_src" | wc -l)
      log "Found $api_count API URLs and $media_count media URLs"

      all_urls=$(printf "%s\n%s\n" "$api_urls_src" "$media_urls_src")
      total_images=$(echo "$all_urls" | wc -l)
      img_counter=1
      
      for image_url in $all_urls; do
        file_name=$(basename "${image_url}")
        ext="${file_name##*.}"
        if [[ "${image_url}" != "${thumbnail}" ]]; then
          url_encoded="${file_name//+/ }"
          url_decoded=$(printf '%b' "${url_encoded//%/\\x}")
          encoded_image_url=${image_url// /%20}
          padded_name=$(printf "%03d" $img_counter)
          if [ "$dry_run" = false ]; then
            printf "  [%03d/%03d] %-50s " "$img_counter" "$total_images" "$encoded_image_url"
            if curl "$encoded_image_url" -so "${directory}/${padded_name}.${ext}"; then
              echo -e "\033[1;32mSuccess\033[0m"
              
              # Convert image to PNG if enabled
              if [ "$convert_to_png" = true ]; then
                convert_to_png "${directory}/${padded_name}.${ext}"
              fi
              
              ((img_counter++))
            else
              echo -e "\033[1;31mFailed\033[0m"
              error "Failed to download ${encoded_image_url}"
            fi
          fi
        fi
      done

      if [ "$dry_run" = false ]; then
        create_cbz "${directory}" "${chapter_name}"
        rm -rf "${directory}"
      fi
    fi
  done
done

success "ALL DONE! Every existing chapter is now in → $folder"

# No temporary JSON files are used; nothing to clean up.
