# Variables
IMAGE_NAME := magi
VERSION ?= develop
REGISTRY := docker.io/alexbruun
REGISTRY_URL := $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Default platforms to build for
PLATFORMS := linux/amd64,linux/arm64

setup-buildx:
	docker buildx create --name mybuilder --use
	docker buildx inspect --bootstrap

# Build the Docker image
build:
	@echo "Building Docker image for platforms: $(PLATFORMS)"
	docker buildx build --progress plain --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) -t $(REGISTRY_URL) --push .

build-develop:
	docker buildx build --progress plain --platform linux/amd64 --build-arg VERSION=$(VERSION) -t local/magi:${VERSION} --load .

# Run the Go build script
# PLATFORM is passed as a positional argument
go-build:
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
all: setup-buildx build
