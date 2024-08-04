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
	./build-release.sh $(PLATFORM) $(VERSION)

# Run all the above commands
all: setup-buildx build
