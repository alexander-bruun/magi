# Variables
IMAGE_NAME := magi
VERSION ?= develop
REGISTRY := docker.io/alexbruun
REGISTRY_URL := $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Default platforms to build for
PLATFORMS := linux/amd64,linux/arm64

# Check if Zig is installed
check-zig:
	@which zig > /dev/null || (echo "Zig is required for CGO builds. Install from https://ziglang.org/download/" && exit 1)

# Show help
help:
	@echo "Available targets:"
	@echo "  setup-buildx     - Setup Docker buildx for multi-arch builds"
	@echo "  build            - Build multi-arch Docker images and push to registry"
	@echo "  build-develop    - Build Docker image for development (linux/amd64 only)"
	@echo "  build-binaries   - Build release binaries for all platforms locally"
	@echo "  build-binary     - Build release binary for specific platform (set PLATFORM variable)"
	@echo "  test             - Run tests"
	@echo "  coverage         - Run tests with coverage"
	@echo "  start-minio      - Start MinIO S3 server for testing"
	@echo "  stop-minio       - Stop MinIO S3 server"
	@echo "  all              - Run setup-buildx, build, and build-binaries"
	@echo ""
	@echo "Examples:"
	@echo "  make build VERSION=1.0.0"
	@echo "  make build-binary PLATFORM=linux/amd64 VERSION=1.0.0"
	@echo "  make build-binaries VERSION=1.0.0"

setup-buildx:
	docker buildx create --name mybuilder --use || docker buildx use mybuilder
	docker buildx inspect --bootstrap

# Build the Docker image for multiple platforms
build:
	@echo "Building Docker image for platforms: $(PLATFORMS)"
	docker buildx build --progress plain --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) -t $(REGISTRY_URL) --push .

# Build Docker image for development (single platform, load locally)
build-develop:
	docker buildx build --progress plain --platform linux/amd64 --build-arg VERSION=$(VERSION) -t local/magi:${VERSION} --load .

# Build all release binaries locally
build-binaries: check-zig
	@echo "Building release binaries for all platforms..."
	./scripts/build-release.sh all $(VERSION)

# Build specific platform binary
build-binary: check-zig
	@echo "Building binary for platform: $(PLATFORM)"
	./scripts/build-release.sh $(PLATFORM) $(VERSION)

# Run the Go build script (legacy, use build-binary instead)
go-build: check-zig
	./scripts/build-release.sh $(PLATFORM) $(VERSION)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests and show coverage per folder
coverage:
	@bash scripts/coverage.sh

# Start MinIO S3 server for testing
start-minio:
	@echo "Starting MinIO S3 server..."
	@docker run -d --name magi-minio \
		-p 9000:9000 \
		-p 9001:9001 \
		-e "MINIO_ACCESS_KEY=magiaccesskey" \
		-e "MINIO_SECRET_KEY=magisecretkey" \
		-v $(PWD)/minio-data:/data \
		minio/minio server /data --console-address ":9001"
	@echo "MinIO started. Creating .env file with credentials..."
	@echo "MAGI_CACHE_BACKEND=s3" > .env
	@echo "MAGI_CACHE_S3_BUCKET=magi-test" >> .env
	@echo "MAGI_CACHE_S3_REGION=us-east-1" >> .env
	@echo "MAGI_CACHE_S3_ENDPOINT=http://localhost:9000" >> .env
	@echo "AWS_ACCESS_KEY_ID=magiaccesskey" >> .env
	@echo "AWS_SECRET_ACCESS_KEY=magisecretkey" >> .env
	@echo "Credentials saved to .env file"

# Stop MinIO S3 server
stop-minio:
	@echo "Stopping MinIO S3 server..."
	@docker stop magi-minio || true
	@docker rm magi-minio || true
	@echo "MinIO stopped"

# Run all the above commands
all: setup-buildx build build-binaries
