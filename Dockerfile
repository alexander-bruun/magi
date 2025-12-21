# Use the official Golang image to create a build artifact.
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS builder

ARG VERSION TARGETOS TARGETARCH TARGETPLATFORM

# Install Zig and WebP development libraries
RUN apk add --no-cache zig libwebp-dev

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Install Node.js and npm for JavaScript obfuscation
RUN apk add --no-cache nodejs npm

# Install `templ` using Go
RUN go install github.com/a-h/templ/cmd/templ@latest

# Generate necessary files using `templ`
RUN templ generate

# Obfuscate JavaScript files for non-develop builds
RUN if [ "$VERSION" != "develop" ]; then \
        mkdir -p assets/js/obfuscated; \
        npx --yes javascript-obfuscator assets/js/magi.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/magi.js || echo "Failed to obfuscate magi.js"; \
        npx --yes javascript-obfuscator assets/js/notifications.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/notifications.js || echo "Failed to obfuscate notifications.js"; \
        npx --yes javascript-obfuscator assets/js/reader.js --options-preset high-obfuscation --debug-protection true --debug-protection-interval 4000 --output assets/js/obfuscated/reader.js || echo "Failed to obfuscate reader.js"; \
        mv assets/js/obfuscated/* assets/js/ 2>/dev/null || true; \
        rm -rf assets/js/obfuscated; \
    fi

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Determine Zig target based on TARGETPLATFORM
RUN case "$TARGETPLATFORM" in \
        "linux/amd64") ZIG_TARGET="x86_64-linux-musl" ;; \
        "linux/arm64") ZIG_TARGET="aarch64-linux-musl" ;; \
        *) echo "Unsupported platform: $TARGETPLATFORM" && exit 1 ;; \
    esac && \
    echo "ZIG_TARGET=$ZIG_TARGET" > /tmp/zig_target


# Build the Go app
RUN . /tmp/zig_target && \
    CGO_ENABLED=1 CC="zig cc -target $ZIG_TARGET" GOARCH=$TARGETARCH GOOS=$TARGETOS go build --tags extended -ldflags="-X 'main.Version=${VERSION}'" -o magi ./main.go

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
