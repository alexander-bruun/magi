#!/bin/bash

source "$(dirname "$0")/scraper_helpers.sh"

set -eE -o functrace

# ===================== CONFIG =====================
dry_run="${dry_run:-false}"
convert_to_png="${convert_to_png:-true}"
domain="https://hivetoons.org"
folder="${folder:-$(cd "$(dirname "$0")" && pwd)/HiveToons}"
default_suffix="[HiveToons]"
proxy="${proxy:-}"
json_file="${json_file:-$(cd "$(dirname "$0")" && pwd)/hivetoons.json}" # Update the file when new stuff: https://api.hivetoons.org/api/query?page=1&perPage=99999 - they have ip abuse protection so can't do live scraping of series list
user_agent="${user_agent:-Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36}"

mkdir -p "$folder"

command -v jq >/dev/null || { echo "ERROR: jq required → sudo apt install jq"; exit 1; }

# Initialize cookies by visiting the main page
init_cookies() {
  log "Initializing cookies..."
  curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.9" -c "$cookie_file" -o /dev/null "$domain"
}

# ===================== JSON PARSERS =====================
extract_series_urls()   { jq -r '.posts[] | select(.isNovel == false) | "/series/" + .slug' "$json_file"; }
extract_series_title()  { local s="${1#/series/}"; jq -r --arg s "$s" '.posts[] | select(.slug == $s) | .postTitle' "$json_file" | head -n1; }

extract_chapter_links() {
  local series_slug="$1"
  local series_url="${domain}/series/${series_slug}"
  local html=$(curl -s -L -H "User-Agent: $user_agent" -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" -H "Accept-Language: en-US,en;q=0.5" -H "Referer: ${domain}/" --compressed "$series_url")
  local tmp_html=$(echo "$html" | tr -d '\n')
  grep -o '\\"slug\\":\\"chapter-[^"]*\\"' <<< "$tmp_html" | sed 's/\\"slug\\":\\"//' | sed 's/\\"//' | sort | uniq
}

# ===================== IMAGE EXTRACTION =====================
extract_image_urls() {
    local url="${domain}${1}"
    for i in {1..3}; do
        local html
        html=$(curl -sf ${proxy:+-x "$proxy"} \
            -H "User-Agent: $user_agent" \
            -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" \
            -H "Accept-Language: en-US,en;q=0.9" \
            -H "Referer: ${domain}/" \
            --compressed "$url" 2>/dev/null || true)

        local tmp_html=$(mktemp)
        echo "$html" | tr -d '\n' > "$tmp_html"
        local imgs
        imgs=$(grep -o 'https://storage\.hivetoon\.com/public/*upload/series/[^"]*\.\(webp\|jpg\|png\|jpeg\|avif\)' "$tmp_html" | sort -u)
        rm "$tmp_html"

        if (( $(wc -l <<<"$imgs") >= 1 )); then
            printf '%s\n' "$imgs"
            return 0
        fi

        sleep 4
    done
    return 1
}

# ===================== MAIN =====================
log "HiveToons → SERIES SCRAPER"

if [[ ! -f "$json_file" ]]; then
  log "Fetching all series data..."
  all_json=$(curl -s "https://api.hivetoons.org/api/query?page=1&perPage=99999")
  echo "$all_json" > "$json_file"
else
  log "Loading series data from cache..."
fi

log "Found $(jq '.posts | length' "$json_file") series in JSON"

readarray -t series_list < <(extract_series_urls)

for series_url in "${series_list[@]}"; do
    title=$(extract_series_title "$series_url")
    [[ -z "$title" ]] && { error "No title → skip"; continue; }

    clean_title=$(html_unescape "$title" | tr -d '<>:"/\\|?*')
    log "Title → $clean_title"

    series_slug="${series_url#/series/}"
    series_dir=$(find "$folder" -maxdepth 1 -type d -name "${clean_title}*" | head -n1 || echo "")
    series_dir="${series_dir:-$folder/$clean_title $default_suffix}"
    mkdir -p "$series_dir"

    # Extract chapter slugs from series page
    chapter_links=$(extract_chapter_links "$series_slug")

    normalize_chapter_numbers "$series_dir" "2"

    # === DOWNLOAD LOOP ===
    for ch_slug in $chapter_links; do
        ch_url="/series/${series_slug}/${ch_slug}"
        num=$(echo "$ch_slug" | sed 's/chapter-//')
        if [[ "$num" =~ ^[0-9]+$ ]]; then
            padded=$(printf "%02d" "$num")
        else
            padded="$num"
        fi
        name="${clean_title} Ch.${padded} ${default_suffix}"

        existing_cbz=$(ls "$series_dir"/*"Ch.${padded}"*.cbz 2>/dev/null | head -n1 || echo "")
        if [[ -n "$existing_cbz" ]]; then
            continue
        fi

        readarray -t imgs < <(extract_image_urls "$ch_url")
        if (( ${#imgs[@]} == 0 )); then
            log "Chapter $num → 404 → skipped"
            continue
        elif (( ${#imgs[@]} == 1 )); then
            log "Chapter $num has only 1 image → skipped"
            continue
        fi

        log "Downloading → Chapter $num [${#imgs[@]} images] → $name"

        tmp=$(mktemp -d)
        downloaded=0
        total=${#imgs[@]}

        for i in "${!imgs[@]}"; do
            idx=$((i+1))
            url="${imgs[$i]// /%20}"
            ext=".${url##*.}"
            file=$(printf "%03d%s" "$idx" "$ext")

            printf "  [%03d/%03d] %-50s " "$idx" "$total" "$url"

            if curl -sfL --connect-timeout 15 --max-time 120 -o "$tmp/$file" -- "$url" >/dev/null 2>&1; then
                echo -e "\033[1;32mSuccess\033[0m"
                downloaded=$((downloaded + 1))

                # Convert image to PNG if enabled
                if $convert_to_png; then
                    convert_to_png "$tmp/$file"
                fi
            else
                echo -e "\033[1;31mFailed\033[0m"
                error "Failed: $url"
                rm -rf "$tmp"
                break
            fi
        done

        (( downloaded != total )) && { warn "Incomplete → skipped"; rm -rf "$tmp"; continue; }

        create_cbz "$tmp" "$series_dir" "$name"
        rm -rf "$tmp"
    done
done

success "ALL DONE! Every existing chapter is now in → $folder"