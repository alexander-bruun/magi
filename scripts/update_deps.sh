#!/bin/bash

# Script to update all Go dependencies in the Magi project

set -e  # Exit on any error

echo "Updating Go dependencies..."

# Update all dependencies to their latest versions
go get -u ./...

# Clean up go.mod and go.sum
go mod tidy

# Verify the build still works
echo "Verifying build..."
go build

echo "Dependencies updated successfully!"