# Use the official Golang image to create a build artifact.
FROM --platform=$BUILDPLATFORM golang:1.26rc2-alpine AS builder

ARG VERSION TARGETOS TARGETARCH TARGETPLATFORM

# Install WebP development libraries
RUN apk add --no-cache libwebp-dev gcc musl-dev

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files first for better caching
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Install Node.js and npm for JavaScript obfuscation
RUN apk add --no-cache nodejs npm

# Install `templ` using Go
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy the source code into the container (after dependencies are downloaded)
COPY . .

# Generate necessary files using `templ`
RUN templ generate

# Obfuscate JavaScript files for non-develop builds
RUN if [ "$VERSION" != "develop" ]; then \
        mkdir -p assets/js/obfuscated; \
        npx --yes javascript-obfuscator assets/js/magi.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/magi.js || echo "Failed to obfuscate magi.js"; \
        npx --yes javascript-obfuscator assets/js/reader.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/reader.js || echo "Failed to obfuscate reader.js"; \
        npx --yes javascript-obfuscator assets/js/browser-challenge.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/browser-challenge.js || echo "Failed to obfuscate browser-challenge.js"; \
        mv assets/js/obfuscated/* assets/js/ 2>/dev/null || true; \
        rm -rf assets/js/obfuscated; \
    fi

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Build the Go app
ARG TARGETPLATFORM
ENV CGO_ENABLED=1
RUN go build --tags extended -ldflags="-X 'main.Version=${VERSION}'" -o magi ./main.go

# Start a new stage from scratch
FROM --platform=$BUILDPLATFORM alpine:3.23.2

# Install ca-certificates (required for HTTPS connections)
RUN apk --no-cache add ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /app/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/magi .

# Expose port 3000 to the outside world
EXPOSE 3000

# Set the entrypoint to the `magi` binary
ENTRYPOINT ["/app/magi"]
