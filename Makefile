# Variables
IMAGE_NAME := magi
TAG := latest
REGISTRY := docker.io/alexander-bruun
REGISTRY_URL := $(REGISTRY)/$(IMAGE_NAME):$(TAG)

# Ensure 'templ' is available and generate necessary files
generate:
	templ generate

# Build the Docker image
build:
	docker build -t $(REGISTRY_URL) .

# Tag the Docker image
tag:
	docker tag $(REGISTRY_URL) $(REGISTRY_URL)

# Push the Docker image to the private registry
push:
	docker push $(REGISTRY_URL)

# Run all the above commands
all: build tag push
