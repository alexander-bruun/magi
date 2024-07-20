#!/bin/bash

# List of OS/ARCH combinations to build
platforms=(
    "windows/amd64"
    "windows/386"
    "linux/amd64"
    "linux/386"
    "darwin/amd64"
    "darwin/arm64" # Apple Silicon
)

# Output directory
output_dir="builds"

# Create the output directory if it doesn't exist
mkdir -p $output_dir

# Loop through each platform
for platform in "${platforms[@]}"
do
    # Split the platform string into OS and ARCH
    IFS="/" read -r -a split <<< "$platform"
    os="${split[0]}"
    arch="${split[1]}"

    # Set the output file name
    output_name="myapp-$os-$arch"

    # Add .exe extension for Windows
    if [ "$os" == "windows" ]; then
        output_name+=".exe"
    fi

    # Build the project
    echo "Building for $os/$arch..."
    env GOOS=$os GOARCH=$arch go build -o "$output_dir/$output_name"

    if [ $? -ne 0 ]; then
        echo "An error has occurred! Aborting the script execution..."
        exit 1
    fi
done

echo "Builds completed successfully!"
