#!/usr/bin/env bash

# ============================================
# Create Dummy Manga Folder Structure (with chapter files)
# ============================================
# Usage:
#   ./create_manga_folders.sh [destination_folder]
#
# Example:
#   ./create_manga_folders.sh "/path/to/MyManga"
#
# If no destination is provided, defaults to ./MangaDummy
# ============================================

# Configurable destination directory
BASE_DIR="${1:-./MangaDummy}"

# Number of dummy chapters per manga
CHAPTER_COUNT=5

# Sample list of top manga (100 titles)
MANGA_LIST=(
  "One Piece"
  "Naruto"
  "Dragon Ball"
  "Bleach"
  "Attack on Titan"
  "Fullmetal Alchemist"
  "Death Note"
  "Hunter x Hunter"
  "My Hero Academia"
  "Demon Slayer: Kimetsu no Yaiba"
  "JoJo's Bizarre Adventure"
  "One-Punch Man"
  "Tokyo Ghoul"
  "Chainsaw Man"
  "Vinland Saga"
  "Black Clover"
  "Fairy Tail"
  "Berserk"
  "Spy×Family"
  "Haikyuu!!"
  "Monster"
  "Vagabond"
  "Gintama"
  "Blue Lock"
  "Oshi no Ko"
  "Kaguya-sama: Love Is War"
  "Dr. Stone"
  "A Silent Voice"
  "Assassination Classroom"
  "Soul Eater"
  "The Promised Neverland"
  "Mob Psycho 100"
  "Tokyo Revengers"
  "Black Butler"
  "Claymore"
  "Dorohedoro"
  "Food Wars!: Shokugeki no Soma"
  "Made in Abyss"
  "Noragami"
  "The Seven Deadly Sins"
  "Akame Ga Kill!"
  "Fruits Basket"
  "Horimiya"
  "Blue Exorcist"
  "Detective Conan"
  "Baki the Grappler"
  "Fist of the North Star"
  "Hajime no Ippo"
  "Beck"
  "Nana"
  "Kimi ni Todoke"
  "Skip Beat!"
  "Solo Leveling"
  "Tower of God"
  "The Breaker"
  "ReLIFE"
  "Pandora Hearts"
  "Gantz"
  "GTO (Great Teacher Onizuka)"
  "Banana Fish"
  "Goodnight Punpun"
  "20th Century Boys"
  "KochiKame"
  "Astro Boy"
  "Akira"
  "Touch"
  "Yuyu Hakusho"
  "Black Lagoon"
  "Komi Can't Communicate"
  "My Dress-Up Darling"
  "Uzumaki"
  "Platinum End"
  "Drifting Dragons"
  "Hell’s Paradise"
  "Erased"
  "Trigun"
  "Parasyte"
  "Usagi Drop"
  "Golden Kamuy"
  "Orange"
  "To Your Eternity"
  "Shaman King"
  "Kuroko’s Basketball"
  "Nisekoi"
  "Rurouni Kenshin"
  "Blame!"
  "Bokurano"
  "Samurai Champloo"
  "Elfen Lied"
  "Inuyasha"
  "Bungo Stray Dogs"
  "Trigun Maximum"
  "Yakitate!! Japan"
  "Blue Giant"
  "Hellsing"
  "Monster #8"
  "Frieren: Beyond Journey’s End"
  "Kaiju No. 8"
  "Sakamoto Days"
)

# ============================================
# Script Execution
# ============================================

echo "📁 Creating dummy manga structure in: $BASE_DIR"
mkdir -p "$BASE_DIR"

for manga in "${MANGA_LIST[@]}"; do
  # sanitize folder name for filesystem safety
  manga_dir="$BASE_DIR/$manga"
  mkdir -p "$manga_dir"

  # create chapter files
  for (( chap=1; chap<=CHAPTER_COUNT; chap++ )); do
    chapter_file="$manga_dir/Chapter_${chap}.cbz"
    echo "This is dummy content for $manga - Chapter $chap" > "$chapter_file"
  done
done

echo "✅ Done! Created dummy manga files under: $BASE_DIR"
