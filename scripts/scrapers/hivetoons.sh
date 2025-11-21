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

mkdir -p "$folder"

command -v jq >/dev/null || { echo "ERROR: jq required → sudo apt install jq"; exit 1; }
[[ -f "$json_file" ]] || { echo "ERROR: $json_file not found"; exit 1; }

# ===================== JSON PARSERS =====================
extract_series_urls()   { jq -r '.posts[] | select(.isNovel == false) | "/series/" + .slug' "$json_file"; }
extract_series_title()  { local s="${1#/series/}"; jq -r --arg s "$s" '.posts[] | select(.slug == $s) | .postTitle' "$json_file" | head -n1; }

extract_chapter_links_from_json() {
    local s="${1#/series/}"
    jq -r --arg s "$s" '
        [.posts[] | select(.slug == $s) | .chapters[].number]
        | sort_by(tonumber)
        | reverse[]
        | "/series/" + $s + "/chapter-" + tostring
    ' "$json_file"
}

# ===================== GET HIGHEST FROM LIVE PAGE =====================
get_highest_live() {
    local series_url="$1"
    curl -sf ${proxy:+-x "$proxy"} \
        -H "User-Agent: Mozilla/5.0" \
        -H "Referer: ${domain}/" \
        "${domain}${series_url}" 2>/dev/null | \
    grep -oE 'chapter-[0-9]+' | grep -oE '[0-9]+' | sort -nr | head -1 || echo "0"
}

# ===================== GENERATE ALL CHAPTER URLS 0 → HIGHEST =====================
generate_all_chapters() {
    local series_url="$1"
    local highest="$2"
    local slug="${series_url#/series/}"
    for ((i=0; i<=highest; i++)); do
        printf '/series/%s/chapter-%d\n' "$slug" "$i"
    done
}

# ===================== IMAGE EXTRACTION =====================
extract_image_urls() {
    local url="${domain}${1}"
    for i in {1..3}; do
        local html
        html=$(curl -sf ${proxy:+-x "$proxy"} \
            -H "User-Agent: Mozilla/5.0" \
            -H "Referer: ${domain}/" \
            --compressed "$url" 2>/dev/null || true)

        local imgs
        imgs=$(echo "$html" |
            grep -oE 'https?://[^[:space:]"\''"]+\.(jpe?g|png|webp|avif)' |
            grep -E '^https?://storage\.hivetoon\.com/public//upload/(series/|20[0-9]{2}/[01][0-9]/[0-3][0-9]/)' |
            sort -u)

        if (( $(wc -l <<<"$imgs") >= 1 )); then
            printf '%s\n' "$imgs"
            return 0
        fi

        sleep 4
    done
    return 1
}

# ===================== MAIN =====================
log "HiveToons → FULL BRUTE-FORCE DOWNLOADER (gets EVERY chapter 0–latest)"
log "Found $(jq '.posts | length' "$json_file") series in JSON"

readarray -t series_list < <(extract_series_urls)

for series_url in "${series_list[@]}"; do
    title=$(extract_series_title "$series_url")
    [[ -z "$title" ]] && { error "No title → skip"; continue; }

    clean_title=$(html_unescape "$title" | tr -d '<>:"/\\|?*')
    log "Title → $clean_title"

    series_dir=$(find "$folder" -maxdepth 1 -type d -name "${clean_title}*" | head -n1 || echo "")
    series_dir="${series_dir:-$folder/$clean_title $default_suffix}"
    mkdir -p "$series_dir"

    # === DETERMINE HIGHEST CHAPTER ===
    readarray -t json_chapters < <(extract_chapter_links_from_json "$series_url")
    highest_json=$(printf '%s\n' "${json_chapters[@]}" | grep -o '[0-9]*$' | sort -nr | head -1 || echo "0")
    highest_live=$(get_highest_live "$series_url")
    highest=$(( highest_json > highest_live ? highest_json : highest_live ))

    log "Highest chapter: $highest (JSON: $highest_json | Live: $highest_live)"
    log "Brute-forcing chapters 0 → $highest"

    mapfile -t all_chapters < <(generate_all_chapters "$series_url" "$highest")
    pad=${#highest}
    (( pad < 2 )) && pad=2
    log "Using $pad-digit padding"

    normalize_chapter_numbers "$series_dir" "$pad"

    # === DOWNLOAD LOOP ===
    for ch_url in "${all_chapters[@]}"; do
        num=$(grep -o '[0-9]*$' <<<"$ch_url")
        padded=$(printf "%0${pad}d" "$num")
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

success "ALL DONE! Every existing chapter from 0 to latest is now in → $folder"