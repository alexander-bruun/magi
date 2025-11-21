#!/bin/bash

# Common helper functions for scraper scripts

# Failure trap
failure() {
  local lineno=$1
  local msg=$2
  echo "Failed at $lineno: $msg"
}
trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

# Logging functions
log()    { echo -e "\033[1;34m[INFO]\033[0m    $*" >&2; }
success(){ echo -e "\033[1;32m[SUCCESS]\033[0m  $*" >&2;  }
warn()   { echo -e "\033[1;33m[WARNING]\033[0m $*" >&2;   }
error()  { echo -e "\033[1;31m[ERROR]\033[0m   $*" >&2;   }

# Image conversion functions
convert_webp_to_png() {
    local input_file="$1"
    local output_file="${input_file%.webp}.png"
    
    if command -v dwebp &> /dev/null; then
        dwebp "$input_file" -o "$output_file" &> /dev/null && rm "$input_file"
    elif command -v convert &> /dev/null; then
        convert "$input_file" "$output_file" && rm "$input_file"
    else
        warn "No WebP conversion tool found (dwebp or ImageMagick convert). Install libwebp-tools or imagemagick."
        return 1
    fi
}

convert_to_png() {
    local input_file="$1"
    local base="${input_file%.*}"
    local ext="${input_file##*.}"
    local output_file="${base}.png"
    
    # Skip if already PNG
    if [[ "$ext" == "png" ]]; then
        return 0
    fi
    
    if [[ "$ext" == "webp" ]]; then
        # Use WebP-specific converter if available
        if command -v dwebp &> /dev/null; then
            dwebp "$input_file" -o "$output_file" &> /dev/null && rm "$input_file"
            return 0
        fi
    fi
    
    # Use ImageMagick for all formats
    if command -v convert &> /dev/null; then
        convert "$input_file" "$output_file" && rm "$input_file"
        return 0
    else
        warn "ImageMagick convert not found. Install imagemagick."
        return 1
    fi
}

# CBZ creation with ordered files
create_cbz() {
    local temp_dir="$1"
    local output_dir_or_name="$2"
    local name=""
    local output_dir=""
    local cbz_file=""

    if [[ $# -eq 3 ]]; then
        # hivetoons style: temp_dir, output_dir, name
        output_dir="$2"
        name="$3"
        cbz_file="$output_dir/$name.cbz"
    else
        # Other scripts: temp_dir, name
        name="$2"
        output_dir=".."
        cbz_file="../$name.cbz"
    fi

    # Store current directory
    local current_dir=$(pwd)

    # Change directory to image files location
    cd "$temp_dir" || { error "Could not change directory to $temp_dir"; exit 1; }
    
    # Create the CBZ file using zip with explicit numeric ordering
    log "Creating CBZ: $cbz_file"
    # Use printf with brace expansion to add files in numeric order, ignoring extension
    # This ensures proper ordering regardless of mixed extensions (jpg, png, webp)
    for i in {001..999}; do
        for ext in jpeg jpg png webp; do
            [ -f "${i}.${ext}" ] && printf '%s\n' "${i}.${ext}"
        done
    done | xargs -r zip -q "$cbz_file" 2>/dev/null || true

    # Revert back to original directory
    cd "$current_dir" || { error "Could not change directory back to $current_dir"; exit 1; }
}

# Normalize chapter numbers in CBZ files
normalize_chapter_numbers() {
  local series_directory="$1"
  local padding_width="$2"

  # If the directory doesn't exist, nothing to do
  if [[ ! -d "$series_directory" ]]; then
    return
  fi

  shopt -s nullglob
  for file in "$series_directory"/*.cbz; do
    local base
    base=$(basename "$file")

    if [[ $base =~ Ch\.([0-9]+) ]]; then
      local oldnum="${BASH_REMATCH[1]}"

      # Force decimal (fix for octal issue) and compute new padded number
      local decimal_num
      decimal_num=$((10#$oldnum))
      local newnum
      newnum=$(printf "%0${padding_width}d" "$decimal_num")

      if [[ "$oldnum" != "$newnum" ]]; then
        local newfile="${base/Ch.${oldnum}/Ch.${newnum}}"
        local newpath="$series_directory/$newfile"

        # If the corrected filename already exists, remove the outdated file
        if [[ -f "$newpath" ]]; then
          log "Removing outdated duplicate: $base (already have $newfile)"
          if [ "$dry_run" = false ]; then
            rm -f "$file"
          fi
        else
          log "Renaming outdated chapter: $base → $newfile"
          if [ "$dry_run" = false ]; then
            mv -n "$file" "$newpath"
          fi
        fi
      fi
    fi
  done
  shopt -u nullglob
}

# Health check function
health_check() {
    local domain="$1"
    local health_check_response=$(curl -s -o /dev/null -w "%{http_code}" "$domain")
    if [ "$health_check_response" -ne 200 ]; then
        error "Health check failed. $domain returned HTTP $health_check_response"
        error "The site may be down or unreachable. Exiting."
        exit 1
    fi
}

# HTML unescape function
html_unescape() {
  local input="$1"

  # Prefer Perl with HTML::Entities if available for full decoding
  if command -v perl &>/dev/null; then
    printf '%s' "$input" | perl -MHTML::Entities -pe 'decode_entities($_)'
    return 0
  fi

  # Fallback: handle common entities via sed
  printf '%s' "$input" | sed \
    -e "s/&#x27;/\\'/g" \
    -e "s/&#39;/\\'/g" \
    -e 's/&amp;/\&/g' \
    -e 's/&quot;/\"/g' \
    -e 's/&lt;/</g' \
    -e 's/&gt;/>/g'
}

# HTML escape function
html_escape() {
  local input="$1"

  # Prefer Perl with HTML::Entities if available for full encoding
  if command -v perl &>/dev/null; then
    printf '%s' "$input" | perl -MHTML::Entities -pe 'encode_entities($_)'
    return 0
  fi

  # Fallback: handle common characters via sed
  printf '%s' "$input" | sed \
    -e 's/&/\&amp;/g' \
    -e 's/</\&lt;/g' \
    -e 's/>/\&gt;/g' \
    -e 's/"/\&quot;/g' \
    -e "s/'/\&#39;/g"
}