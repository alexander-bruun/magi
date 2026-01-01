#!/bin/bash

# Exit on any error
set -e

# List of OS/ARCH combinations to build
platforms=(
    "windows/amd64"
    "windows/arm64"
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# Output directory
output_dir="builds"

# Create the output directory if it doesn't exist
mkdir -p "$output_dir"

# Function to create sha256sum
create_checksum() {
    local file=$1
    sha256sum "$file" > "${file}.sha256"
}

# Function to build for a specific platform
build_for_platform() {
    local os=$1
    local arch=$2
    local version=$3

    # Determine Zig target
    local zig_target
    case "$os/$arch" in
        linux/amd64) zig_target="x86_64-linux-gnu" ;;
        linux/arm64) zig_target="aarch64-linux-gnu" ;;
        windows/amd64) zig_target="x86_64-windows-gnu" ;;
        windows/arm64) zig_target="aarch64-windows-gnu" ;;
        darwin/amd64) zig_target="x86_64-macos-none" ;;
        darwin/arm64) zig_target="aarch64nu" ;;
        windows/arm64) zig_target="aarch64-windows-gnu" ;;
        darwin/amd64) zig_target="x86_64-macos-none" ;;
        darwin/arm64) zig_target="aarch64-macos-none" ;;
        *) echo "Unsupported platform: $os/$arch" && exit 1 ;;
    esac

    # Set the output file name
    output_name="magi-$os-$arch"

    # Add .exe extension for Windows
    if [ "$os" == "windows" ]; then
        output_name+=".exe"
    fi

    # Build the project
    echo "Building for $os/$arch (${version})..."
    if [ "$os" == "darwin" ]; then
        # macOS builds without extended tags (PNG fallback for WebP)
        env CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags="-X 'main.Version=${version}'" -o "$output_dir/$output_name"
    else
        # Use Zig for cross-compilation with CGO and extended tags for WebP support
        env CGO_ENABLED=1 CC="zig cc -target $zig_target" GOOS=$os GOARCH=$arch go build --tags extended -ldflags="-X 'main.Version=${version}'" -o "$output_dir/$output_name"
    fi

    # Compress and create checksum based on OS
    case "$os" in
        linux|darwin)
            tar -czvf "$output_dir/${output_name}.tar.gz" -C "$output_dir" "$output_name"
            rm "$output_dir/$output_name"
            create_checksum "$output_dir/${output_name}.tar.gz"
            ;;
        windows)
            zip -j "$output_dir/${output_name}.zip" "$output_dir/$output_name"
            rm "$output_dir/$output_name"
            create_checksum "$output_dir/${output_name}.zip"
            ;;
        *)
            echo "Unsupported OS: $os"
            ;;
    esac
}

# Main script logic
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <os/arch|all> <version>"
    exit 1
fi

param=$1
version=$2

# Obfuscate JavaScript files for non-develop builds (only once)
if [ "$version" != "develop" ]; then
    echo "Obfuscating JavaScript files..."
    if command -v node &> /dev/null && command -v npm &> /dev/null; then
        mkdir -p assets/js/obfuscated
        npx --yes javascript-obfuscator assets/js/magi.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/magi.js || echo "Failed to obfuscate magi.js"
        npx --yes javascript-obfuscator assets/js/reader.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/reader.js || echo "Failed to obfuscate reader.js"
        npx --yes javascript-obfuscator assets/js/browser-challenge.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/browser-challenge.js || echo "Failed to obfuscate browser-challenge.js"
        mv assets/js/obfuscated/* assets/js/ 2>/dev/null || true
        rm -rf assets/js/obfuscated
    else
        echo "Warning: Node.js or npm not found. Skipping JavaScript obfuscation."
    fi
fi

if [ "$param" == "all" ]; then
    # Build for all platforms
    for platform in "${platforms[@]}"; do
        IFS="/" read -r -a split <<< "$platform"
        os="${split[0]}"
        arch="${split[1]}"

        build_for_platform "$os" "$arch" "$version"
    done
else
    # Validate and build for a specific platform
    matched=false
    for platform in "${platforms[@]}"; do
        if [ "$param" == "$platform" ]; then
            IFS="/" read -r -a split <<< "$platform"
            os="${split[0]}"
            arch="${split[1]}"
            build_for_platform "$os" "$arch" "$version"
            matched=true
            break
        fi
    done

    if [ "$matched" = false ]; then
        echo "Unsupported platform: $param"
        echo "Supported platforms: ${platforms[@]}"
        exit 1
    fi
fi

echo "Builds completed successfully!"
