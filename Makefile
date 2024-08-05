# Variables
IMAGE_NAME := magi
TAG := latest
REGISTRY := docker.io/alexbruun
REGISTRY_URL := $(REGISTRY)/$(IMAGE_NAME):$(TAG)
PLATFORMS := linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6,linux/s390x,linux/ppc64le,linux/ppc64le

# Set up Docker Buildx
setup-buildx:
	docker buildx create --name mybuilder --use
	docker buildx inspect --bootstrap

# Build the Docker image
build:
	docker buildx build --platform $(PLATFORMS) -t $(REGISTRY_URL) --push .

# Tag the Docker image
tag:
	docker tag $(REGISTRY_URL) $(REGISTRY_URL)

# Push the Docker image to the private registry
push:
	docker push $(REGISTRY_URL)

# Run all the above commands
all: setup-buildx build tag push
