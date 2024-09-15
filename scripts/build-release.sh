#!/bin/bash

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

    # Set the output file name
    output_name="magi-$os-$arch"

    # Add .exe extension for Windows
    if [ "$os" == "windows" ]; then
        output_name+=".exe"
    fi

    # Build the project
    echo "Building for $os/$arch (${version})..."
    env GOOS=$os GOARCH=$arch go build -ldflags="-X 'main.Version=${version}'" -o "$output_dir/$output_name"

    if [ $? -ne 0 ]; then
        echo "An error has occurred! Aborting the script execution..."
        exit 1
    fi

    # Compress and create checksum based on OS
    case "$os" in
        linux)
            tar -czvf "$output_dir/${output_name}.tar.gz" -C "$output_dir" "$output_name"
            rm "$output_dir/$output_name"
            create_checksum "$output_dir/${output_name}.tar.gz"
            ;;
        windows)
            zip -j "$output_dir/${output_name}.zip" "$output_dir/$output_name"
            rm "$output_dir/$output_name"
            create_checksum "$output_dir/${output_name}.zip"
            ;;
        darwin|freebsd|openbsd|netbsd|aix)
            tar -czvf "$output_dir/${output_name}.tar.gz" -C "$output_dir" "$output_name"
            rm "$output_dir/$output_name"
            create_checksum "$output_dir/${output_name}.tar.gz"
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
