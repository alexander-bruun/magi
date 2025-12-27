# Variables
VERSION ?= develop

# Check if Zig is installed
check-zig:
	@which zig > /dev/null || (echo "Zig is required for CGO builds. Install from https://ziglang.org/download/" && exit 1)

# Show help
help:
	@echo "Magi Build System"
	@echo "================="
	@echo ""
	@echo "Build Targets:"
	@echo "  build-binaries     - Build release binaries for all platforms"
	@echo "  build-binary       - Build release binary for specific platform (PLATFORM=linux/amd64)"
	@echo "  build-docker       - Build Docker image for current platform"
	@echo "  build-docker-multi - Build multi-platform Docker image (amd64 + arm64)"
	@echo ""
	@echo "Development:"
	@echo "  test               - Run tests"
	@echo "  coverage           - Run tests with coverage"
	@echo ""
	@echo "Services:"
	@echo "  start-minio        - Start MinIO S3 server for testing"
	@echo "  stop-minio         - Stop MinIO S3 server"
	@echo ""
	@echo "Utilities:"
	@echo "  all                - Run build-binaries and test"
	@echo "  help               - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make build-binary PLATFORM=linux/amd64 VERSION=1.0.0"
	@echo "  make build-binaries VERSION=1.0.0"
	@echo "  make build-docker-multi"

# Build Docker image for current platform
build-docker:
	@echo "Building Docker image for current platform..."
	docker build -t magi:$(VERSION) .

# Build multi-platform Docker image
build-docker-multi:
	@echo "Building multi-platform Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t magi:$(VERSION) --load .

# Build all release binaries locally
build-binaries: check-zig
	@echo "Building release binaries for all platforms locally..."
	./scripts/build-release.sh all $(VERSION)

# Build specific platform binary
build-binary: check-zig
	@echo "Building binary for platform: $(PLATFORM)"
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
all: build-binaries test
